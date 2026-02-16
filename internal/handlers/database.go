package handlers

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

type DatabaseHandler struct {
	db *gorm.DB
}

func NewDatabaseHandler(db *gorm.DB) *DatabaseHandler {
	return &DatabaseHandler{db: db}
}

// validTableName checks that a table name is safe (alphanumeric + underscore only).
var validTableNameRegex = regexp.MustCompile(`^[a-zA-Z_][a-zA-Z0-9_]*$`)

// getTableNames returns a whitelist of actual table names from information_schema.
func (h *DatabaseHandler) getTableNames() (map[string]bool, error) {
	var tables []struct {
		TableName string
	}
	err := h.db.Raw("SELECT table_name FROM information_schema.tables WHERE table_schema = 'public'").Scan(&tables).Error
	if err != nil {
		return nil, err
	}

	result := make(map[string]bool)
	for _, t := range tables {
		result[t.TableName] = true
	}
	return result, nil
}

// ListTables returns all tables in the public schema.
func (h *DatabaseHandler) ListTables(c *fiber.Ctx) error {
	var tables []struct {
		TableName string `json:"table_name"`
	}

	err := h.db.Raw(`
		SELECT table_name
		FROM information_schema.tables
		WHERE table_schema = 'public'
		ORDER BY table_name
	`).Scan(&tables).Error

	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error":   true,
			"message": "Failed to list tables: " + err.Error(),
		})
	}

	// Also get row counts for each table
	type TableInfo struct {
		Name     string `json:"name"`
		RowCount int64  `json:"row_count"`
	}

	var tableInfos []TableInfo
	for _, t := range tables {
		var count int64
		h.db.Raw(fmt.Sprintf("SELECT COUNT(*) FROM %q", t.TableName)).Scan(&count)
		tableInfos = append(tableInfos, TableInfo{
			Name:     t.TableName,
			RowCount: count,
		})
	}

	return c.JSON(fiber.Map{"tables": tableInfos})
}

// GetTableRows returns paginated rows from a specific table.
func (h *DatabaseHandler) GetTableRows(c *fiber.Ctx) error {
	tableName := c.Params("name")

	// Validate table name format
	if !validTableNameRegex.MatchString(tableName) {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error":   true,
			"message": "Invalid table name",
		})
	}

	// Validate against whitelist of actual tables
	validTables, err := h.getTableNames()
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error":   true,
			"message": "Failed to validate table name",
		})
	}

	if !validTables[tableName] {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error":   true,
			"message": "Table not found",
		})
	}

	limit := c.QueryInt("limit", 50)
	offset := c.QueryInt("offset", 0)
	if limit < 1 || limit > 500 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}

	// Get total count
	var total int64
	h.db.Raw(fmt.Sprintf("SELECT COUNT(*) FROM %q", tableName)).Scan(&total)

	// Get rows — use quoted identifier to prevent injection
	var rows []map[string]interface{}
	err = h.db.Raw(fmt.Sprintf("SELECT * FROM %q LIMIT ? OFFSET ?", tableName), limit, offset).Scan(&rows).Error
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error":   true,
			"message": "Failed to query table: " + err.Error(),
		})
	}

	// Get column info
	var columns []struct {
		ColumnName string `json:"column_name"`
		DataType   string `json:"data_type"`
		IsNullable string `json:"is_nullable"`
	}
	h.db.Raw(`
		SELECT column_name, data_type, is_nullable
		FROM information_schema.columns
		WHERE table_schema = 'public' AND table_name = ?
		ORDER BY ordinal_position
	`, tableName).Scan(&columns)

	return c.JSON(fiber.Map{
		"table":   tableName,
		"rows":    rows,
		"columns": columns,
		"total":   total,
		"limit":   limit,
		"offset":  offset,
	})
}

// ExecuteQuery executes a read-only SQL query.
func (h *DatabaseHandler) ExecuteQuery(c *fiber.Ctx) error {
	var req struct {
		Query string `json:"query"`
	}
	if err := c.BodyParser(&req); err != nil || req.Query == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error":   true,
			"message": "Query is required",
		})
	}

	// Basic safety check — disallow mutation keywords
	upper := strings.ToUpper(strings.TrimSpace(req.Query))
	disallowed := []string{"INSERT", "UPDATE", "DELETE", "DROP", "ALTER", "CREATE", "TRUNCATE", "GRANT", "REVOKE", "COPY"}
	for _, kw := range disallowed {
		if strings.Contains(upper, kw) {
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
				"error":   true,
				"message": fmt.Sprintf("Mutation queries are not allowed (found %s)", kw),
			})
		}
	}

	// Execute in a read-only transaction
	var rows []map[string]interface{}
	err := h.db.Transaction(func(tx *gorm.DB) error {
		// Set transaction to read-only
		if err := tx.Exec("SET TRANSACTION READ ONLY").Error; err != nil {
			return err
		}
		return tx.Raw(req.Query).Scan(&rows).Error
	})

	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error":   true,
			"message": "Query failed: " + err.Error(),
		})
	}

	return c.JSON(fiber.Map{
		"query":     req.Query,
		"rows":      rows,
		"row_count": len(rows),
	})
}

// GetDatabaseStats returns database statistics.
func (h *DatabaseHandler) GetDatabaseStats(c *fiber.Ctx) error {
	// Database size
	var dbSize string
	h.db.Raw("SELECT pg_size_pretty(pg_database_size(current_database()))").Scan(&dbSize)

	// Active connections
	var activeConnections int64
	h.db.Raw("SELECT count(*) FROM pg_stat_activity WHERE state = 'active'").Scan(&activeConnections)

	// Total connections
	var totalConnections int64
	h.db.Raw("SELECT count(*) FROM pg_stat_activity").Scan(&totalConnections)

	// Table count
	var tableCount int64
	h.db.Raw("SELECT count(*) FROM information_schema.tables WHERE table_schema = 'public'").Scan(&tableCount)

	// Database version
	var version string
	h.db.Raw("SELECT version()").Scan(&version)

	// Uptime
	var uptime string
	h.db.Raw("SELECT now() - pg_postmaster_start_time()").Scan(&uptime)

	// Table sizes
	type TableSize struct {
		Name      string `json:"name"`
		Size      string `json:"size"`
		RowCount  int64  `json:"row_count"`
		TotalSize string `json:"total_size"`
	}
	var tableSizes []TableSize
	h.db.Raw(`
		SELECT
			t.table_name as name,
			pg_size_pretty(pg_total_relation_size(quote_ident(t.table_name))) as total_size,
			pg_size_pretty(pg_relation_size(quote_ident(t.table_name))) as size
		FROM information_schema.tables t
		WHERE t.table_schema = 'public'
		ORDER BY pg_total_relation_size(quote_ident(t.table_name)) DESC
	`).Scan(&tableSizes)

	return c.JSON(fiber.Map{
		"database_size":      dbSize,
		"active_connections": activeConnections,
		"total_connections":  totalConnections,
		"table_count":        tableCount,
		"version":            version,
		"uptime":             uptime,
		"table_sizes":        tableSizes,
	})
}
