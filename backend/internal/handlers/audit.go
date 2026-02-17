package handlers

import (
	"encoding/json"
	"strconv"

	"github.com/ahmetk3436/bastion/internal/models"
	"github.com/gofiber/fiber/v2"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

type AuditHandler struct {
	db *gorm.DB
}

func NewAuditHandler(db *gorm.DB) *AuditHandler {
	return &AuditHandler{db: db}
}

// ListAuditLogs returns paginated audit logs, filterable by actor and action.
func (h *AuditHandler) ListAuditLogs(c *fiber.Ctx) error {
	page, _ := strconv.Atoi(c.Query("page", "1"))
	perPage, _ := strconv.Atoi(c.Query("per_page", "50"))
	actor := c.Query("actor", "")
	action := c.Query("action", "")

	if page < 1 {
		page = 1
	}
	if perPage < 1 || perPage > 200 {
		perPage = 50
	}

	query := h.db.Model(&models.AuditLog{})

	if actor != "" {
		query = query.Where("actor = ?", actor)
	}
	if action != "" {
		query = query.Where("action = ?", action)
	}

	var total int64
	query.Count(&total)

	var logs []models.AuditLog
	if err := query.Order("created_at DESC").
		Offset((page - 1) * perPage).
		Limit(perPage).
		Find(&logs).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error":   true,
			"message": "Failed to list audit logs",
		})
	}

	return c.JSON(fiber.Map{
		"logs":     logs,
		"total":    total,
		"page":     page,
		"per_page": perPage,
	})
}

// CreateAuditLog is an internal helper to record audit entries.
func CreateAuditLog(db *gorm.DB, actor, action, target string, details map[string]interface{}) error {
	var detailsJSON datatypes.JSON
	if details != nil {
		b, err := json.Marshal(details)
		if err == nil {
			detailsJSON = datatypes.JSON(b)
		}
	}

	log := models.AuditLog{
		Actor:   actor,
		Action:  action,
		Target:  target,
		Details: detailsJSON,
	}

	return db.Create(&log).Error
}
