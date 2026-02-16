package handlers

import (
	"crypto/tls"
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"

	"github.com/ahmetk3436/bastion/internal/models"
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type MonitorHandler struct {
	db *gorm.DB
}

func NewMonitorHandler(db *gorm.DB) *MonitorHandler {
	return &MonitorHandler{db: db}
}

// ListMonitors returns all monitors.
func (h *MonitorHandler) ListMonitors(c *fiber.Ctx) error {
	var monitors []models.Monitor
	if err := h.db.Order("created_at DESC").Find(&monitors).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error":   true,
			"message": "Failed to list monitors",
		})
	}
	return c.JSON(fiber.Map{"monitors": monitors})
}

// CreateMonitor creates a new uptime monitor.
func (h *MonitorHandler) CreateMonitor(c *fiber.Ctx) error {
	var req struct {
		Name            string `json:"name"`
		URL             string `json:"url"`
		Type            string `json:"type"`
		Method          string `json:"method"`
		IntervalSeconds int    `json:"interval_seconds"`
		TimeoutMs       int    `json:"timeout_ms"`
		ExpectedStatus  int    `json:"expected_status"`
	}

	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error":   true,
			"message": "Invalid request body",
		})
	}

	if req.Name == "" || req.URL == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error":   true,
			"message": "Name and URL are required",
		})
	}

	// Validate URL has protocol
	if !strings.HasPrefix(req.URL, "http://") && !strings.HasPrefix(req.URL, "https://") && !strings.HasPrefix(req.URL, "tcp://") {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error":   true,
			"message": "URL must start with http://, https://, or tcp://",
		})
	}

	monitor := models.Monitor{
		Name: req.Name,
		URL:  req.URL,
	}

	if req.Type != "" {
		monitor.Type = req.Type
	}
	if req.Method != "" {
		monitor.Method = req.Method
	}
	if req.IntervalSeconds > 0 {
		monitor.IntervalSeconds = req.IntervalSeconds
	}
	if req.TimeoutMs > 0 {
		monitor.TimeoutMs = req.TimeoutMs
	}
	if req.ExpectedStatus > 0 {
		monitor.ExpectedStatus = req.ExpectedStatus
	}

	if err := h.db.Create(&monitor).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error":   true,
			"message": "Failed to create monitor",
		})
	}

	return c.Status(fiber.StatusCreated).JSON(monitor)
}

// GetMonitor returns a single monitor with recent pings.
func (h *MonitorHandler) GetMonitor(c *fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error":   true,
			"message": "Invalid monitor ID",
		})
	}

	var monitor models.Monitor
	if err := h.db.First(&monitor, "id = ?", id).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error":   true,
			"message": "Monitor not found",
		})
	}

	// Get recent pings (last 50)
	var pings []models.MonitorPing
	h.db.Where("monitor_id = ?", id).Order("checked_at DESC").Limit(50).Find(&pings)

	return c.JSON(fiber.Map{
		"monitor": monitor,
		"pings":   pings,
	})
}

// DeleteMonitor soft-deletes a monitor.
func (h *MonitorHandler) DeleteMonitor(c *fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error":   true,
			"message": "Invalid monitor ID",
		})
	}

	if err := h.db.Delete(&models.Monitor{}, "id = ?", id).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error":   true,
			"message": "Failed to delete monitor",
		})
	}

	return c.JSON(fiber.Map{"message": "Monitor deleted"})
}

// ToggleMonitor toggles the enabled state of a monitor.
func (h *MonitorHandler) ToggleMonitor(c *fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error":   true,
			"message": "Invalid monitor ID",
		})
	}

	var monitor models.Monitor
	if err := h.db.First(&monitor, "id = ?", id).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error":   true,
			"message": "Monitor not found",
		})
	}

	monitor.Enabled = !monitor.Enabled
	h.db.Save(&monitor)

	return c.JSON(fiber.Map{
		"message": fmt.Sprintf("Monitor %s", map[bool]string{true: "enabled", false: "disabled"}[monitor.Enabled]),
		"enabled": monitor.Enabled,
	})
}

// GetMonitorPings returns paginated pings for a monitor.
func (h *MonitorHandler) GetMonitorPings(c *fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error":   true,
			"message": "Invalid monitor ID",
		})
	}

	page, _ := strconv.Atoi(c.Query("page", "1"))
	perPage, _ := strconv.Atoi(c.Query("per_page", "50"))
	if page < 1 {
		page = 1
	}
	if perPage < 1 || perPage > 200 {
		perPage = 50
	}

	var total int64
	h.db.Model(&models.MonitorPing{}).Where("monitor_id = ?", id).Count(&total)

	var pings []models.MonitorPing
	h.db.Where("monitor_id = ?", id).
		Order("checked_at DESC").
		Offset((page - 1) * perPage).
		Limit(perPage).
		Find(&pings)

	return c.JSON(fiber.Map{
		"pings":    pings,
		"total":    total,
		"page":     page,
		"per_page": perPage,
	})
}

// CheckSSL connects to a domain and returns SSL certificate info.
func (h *MonitorHandler) CheckSSL(c *fiber.Ctx) error {
	var req struct {
		Domain string `json:"domain"`
	}
	if err := c.BodyParser(&req); err != nil || req.Domain == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error":   true,
			"message": "Domain is required",
		})
	}

	// Strip protocol if provided
	domain := req.Domain
	domain = strings.TrimPrefix(domain, "https://")
	domain = strings.TrimPrefix(domain, "http://")
	domain = strings.TrimSuffix(domain, "/")
	// Remove port if present
	if idx := strings.Index(domain, ":"); idx != -1 {
		domain = domain[:idx]
	}

	conn, err := tls.DialWithDialer(
		&net.Dialer{Timeout: 10 * time.Second},
		"tcp",
		domain+":443",
		&tls.Config{
			InsecureSkipVerify: false,
		},
	)
	if err != nil {
		return c.Status(fiber.StatusBadGateway).JSON(fiber.Map{
			"error":   true,
			"message": "TLS connection failed: " + err.Error(),
		})
	}
	defer conn.Close()

	certs := conn.ConnectionState().PeerCertificates
	if len(certs) == 0 {
		return c.Status(fiber.StatusBadGateway).JSON(fiber.Map{
			"error":   true,
			"message": "No certificates found",
		})
	}

	cert := certs[0]
	now := time.Now()
	daysRemaining := int(cert.NotAfter.Sub(now).Hours() / 24)

	// Save or update in DB
	sslCert := models.SSLCert{}
	result := h.db.Where("domain = ?", domain).First(&sslCert)
	if result.Error != nil {
		// Create new
		sslCert = models.SSLCert{
			Domain:        domain,
			Issuer:        cert.Issuer.CommonName,
			ValidFrom:     cert.NotBefore,
			ValidTo:       cert.NotAfter,
			DaysRemaining: daysRemaining,
			LastCheckedAt: &now,
		}
		h.db.Create(&sslCert)
	} else {
		// Update existing
		h.db.Model(&sslCert).Updates(map[string]interface{}{
			"issuer":          cert.Issuer.CommonName,
			"valid_from":      cert.NotBefore,
			"valid_to":        cert.NotAfter,
			"days_remaining":  daysRemaining,
			"last_checked_at": now,
		})
	}

	return c.JSON(fiber.Map{
		"domain":         domain,
		"issuer":         cert.Issuer.CommonName,
		"subject":        cert.Subject.CommonName,
		"valid_from":     cert.NotBefore,
		"valid_to":       cert.NotAfter,
		"days_remaining": daysRemaining,
		"dns_names":      cert.DNSNames,
		"is_valid":       now.After(cert.NotBefore) && now.Before(cert.NotAfter),
	})
}

// ListSSLCerts returns all tracked SSL certificates.
func (h *MonitorHandler) ListSSLCerts(c *fiber.Ctx) error {
	var certs []models.SSLCert
	if err := h.db.Order("days_remaining ASC").Find(&certs).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error":   true,
			"message": "Failed to list SSL certificates",
		})
	}
	return c.JSON(fiber.Map{"ssl_certs": certs})
}
