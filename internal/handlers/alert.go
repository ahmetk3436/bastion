package handlers

import (
	"time"

	"github.com/ahmetk3436/bastion/internal/models"
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type AlertHandler struct {
	db *gorm.DB
}

func NewAlertHandler(db *gorm.DB) *AlertHandler {
	return &AlertHandler{db: db}
}

// ListAlertRules returns all alert rules.
func (h *AlertHandler) ListAlertRules(c *fiber.Ctx) error {
	var rules []models.AlertRule
	if err := h.db.Order("created_at DESC").Find(&rules).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error":   true,
			"message": "Failed to list alert rules",
		})
	}
	return c.JSON(fiber.Map{"rules": rules})
}

// CreateAlertRule creates a new alert rule.
func (h *AlertHandler) CreateAlertRule(c *fiber.Ctx) error {
	var req struct {
		Name                string  `json:"name"`
		Type                string  `json:"type"`
		Metric              string  `json:"metric"`
		Operator            string  `json:"operator"`
		Threshold           float64 `json:"threshold"`
		DurationSeconds     int     `json:"duration_seconds"`
		NotificationChannel string  `json:"notification_channel"`
	}

	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error":   true,
			"message": "Invalid request body",
		})
	}

	if req.Name == "" || req.Type == "" || req.Metric == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error":   true,
			"message": "Name, type, and metric are required",
		})
	}

	// Validate operator
	validOps := map[string]bool{">": true, "<": true, ">=": true, "<=": true, "==": true}
	if req.Operator != "" && !validOps[req.Operator] {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error":   true,
			"message": "Invalid operator. Must be: >, <, >=, <=, ==",
		})
	}

	rule := models.AlertRule{
		Name:      req.Name,
		Type:      req.Type,
		Metric:    req.Metric,
		Threshold: req.Threshold,
	}

	if req.Operator != "" {
		rule.Operator = req.Operator
	}
	if req.DurationSeconds > 0 {
		rule.DurationSeconds = req.DurationSeconds
	}
	if req.NotificationChannel != "" {
		rule.NotificationChannel = req.NotificationChannel
	}

	if err := h.db.Create(&rule).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error":   true,
			"message": "Failed to create alert rule",
		})
	}

	return c.Status(fiber.StatusCreated).JSON(rule)
}

// DeleteAlertRule soft-deletes an alert rule.
func (h *AlertHandler) DeleteAlertRule(c *fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error":   true,
			"message": "Invalid rule ID",
		})
	}

	if err := h.db.Delete(&models.AlertRule{}, "id = ?", id).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error":   true,
			"message": "Failed to delete alert rule",
		})
	}

	return c.JSON(fiber.Map{"message": "Alert rule deleted"})
}

// ListAlerts returns alerts, optionally filtered by status.
func (h *AlertHandler) ListAlerts(c *fiber.Ctx) error {
	status := c.Query("status", "")

	query := h.db.Order("created_at DESC")
	if status != "" {
		query = query.Where("status = ?", status)
	}

	var alerts []models.Alert
	if err := query.Limit(200).Find(&alerts).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error":   true,
			"message": "Failed to list alerts",
		})
	}

	return c.JSON(fiber.Map{"alerts": alerts})
}

// AcknowledgeAlert sets the acknowledged_at timestamp.
func (h *AlertHandler) AcknowledgeAlert(c *fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error":   true,
			"message": "Invalid alert ID",
		})
	}

	var alert models.Alert
	if err := h.db.First(&alert, "id = ?", id).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error":   true,
			"message": "Alert not found",
		})
	}

	now := time.Now()
	alert.Status = "acknowledged"
	alert.AcknowledgedAt = &now
	h.db.Save(&alert)

	return c.JSON(fiber.Map{
		"message": "Alert acknowledged",
		"alert":   alert,
	})
}

// ResolveAlert sets the resolved_at timestamp.
func (h *AlertHandler) ResolveAlert(c *fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error":   true,
			"message": "Invalid alert ID",
		})
	}

	var alert models.Alert
	if err := h.db.First(&alert, "id = ?", id).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error":   true,
			"message": "Alert not found",
		})
	}

	now := time.Now()
	alert.Status = "resolved"
	alert.ResolvedAt = &now
	h.db.Save(&alert)

	return c.JSON(fiber.Map{
		"message": "Alert resolved",
		"alert":   alert,
	})
}
