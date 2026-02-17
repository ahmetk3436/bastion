package handlers

import (
	"strconv"
	"time"

	"github.com/ahmetk3436/bastion/internal/models"
	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

type RemoteConfigHandler struct {
	db *gorm.DB
}

func NewRemoteConfigHandler(db *gorm.DB) *RemoteConfigHandler {
	return &RemoteConfigHandler{db: db}
}

// GetConfig returns all config as a flat JSON object (public, no auth)
func (h *RemoteConfigHandler) GetConfig(c *fiber.Ctx) error {
	var configs []models.RemoteConfig
	h.db.Find(&configs)

	result := make(map[string]interface{})
	var maxUpdated time.Time

	for _, cfg := range configs {
		switch cfg.Type {
		case "bool":
			result[cfg.Key] = cfg.Value == "true" || cfg.Value == "1"
		case "int":
			if v, err := strconv.Atoi(cfg.Value); err == nil {
				result[cfg.Key] = v
			} else {
				result[cfg.Key] = cfg.Value
			}
		default:
			result[cfg.Key] = cfg.Value
		}
		if cfg.UpdatedAt.After(maxUpdated) {
			maxUpdated = cfg.UpdatedAt
		}
	}

	result["config_version"] = maxUpdated.Unix()

	// Set cache headers
	c.Set("Cache-Control", "public, max-age=60")
	if !maxUpdated.IsZero() {
		c.Set("Last-Modified", maxUpdated.UTC().Format("Mon, 02 Jan 2006 15:04:05 GMT"))
	}

	return c.JSON(result)
}

// GetConfigKey returns a single config value
func (h *RemoteConfigHandler) GetConfigKey(c *fiber.Ctx) error {
	key := c.Params("key")

	var cfg models.RemoteConfig
	if err := h.db.Where("key = ?", key).First(&cfg).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error":   true,
			"message": "Config key not found: " + key,
		})
	}

	return c.JSON(fiber.Map{
		"key":        cfg.Key,
		"value":      cfg.Value,
		"type":       cfg.Type,
		"updated_at": cfg.UpdatedAt,
	})
}

// SetConfigKey creates or updates a config value
func (h *RemoteConfigHandler) SetConfigKey(c *fiber.Ctx) error {
	key := c.Params("key")

	var req struct {
		Value string `json:"value"`
		Type  string `json:"type"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error":   true,
			"message": "Invalid request body",
		})
	}

	if req.Type == "" {
		req.Type = "string"
	}

	var cfg models.RemoteConfig
	result := h.db.Where("key = ?", key).First(&cfg)

	if result.Error != nil {
		// Create new
		cfg = models.RemoteConfig{
			Key:   key,
			Value: req.Value,
			Type:  req.Type,
		}
		h.db.Create(&cfg)
	} else {
		// Update existing
		h.db.Model(&cfg).Updates(map[string]interface{}{
			"value":      req.Value,
			"type":       req.Type,
			"updated_at": time.Now(),
		})
	}

	return c.JSON(fiber.Map{
		"key":        key,
		"value":      req.Value,
		"type":       req.Type,
		"updated_at": cfg.UpdatedAt,
	})
}

// DeleteConfigKey removes a config key
func (h *RemoteConfigHandler) DeleteConfigKey(c *fiber.Ctx) error {
	key := c.Params("key")
	h.db.Where("key = ?", key).Delete(&models.RemoteConfig{})
	return c.JSON(fiber.Map{"message": "Config key deleted: " + key})
}

// SeedDefaults inserts default config values if they don't exist
func (h *RemoteConfigHandler) SeedDefaults() {
	defaults := []models.RemoteConfig{
		{Key: "api_url", Value: "http://89.47.113.196:8097/api", Type: "string"},
		{Key: "ws_url", Value: "ws://89.47.113.196:8097/api", Type: "string"},
		{Key: "sentry_dsn", Value: "http://4b7f6f49adf5409398c66214920211f8@89.47.113.196:8096/14", Type: "string"},
		{Key: "feature_terminal", Value: "true", Type: "bool"},
		{Key: "feature_ai", Value: "true", Type: "bool"},
		{Key: "feature_cron", Value: "true", Type: "bool"},
		{Key: "feature_docker", Value: "true", Type: "bool"},
		{Key: "feature_monitors", Value: "true", Type: "bool"},
		{Key: "app_name", Value: "Bastion", Type: "string"},
		{Key: "app_version", Value: "2.2.0", Type: "string"},
		{Key: "min_app_version", Value: "2.0.0", Type: "string"},
		{Key: "maintenance_mode", Value: "false", Type: "bool"},
		{Key: "announcement", Value: "", Type: "string"},
	}

	for _, d := range defaults {
		var existing models.RemoteConfig
		if h.db.Where("key = ?", d.Key).First(&existing).Error != nil {
			h.db.Create(&d)
		}
	}
}
