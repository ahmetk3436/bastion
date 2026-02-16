package handlers

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/ahmetk3436/bastion/internal/config"
	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

var startTime = time.Now()
var Version = "1.0.0"

type SystemHandler struct {
	db     *gorm.DB
	cfg    *config.Config
	client *http.Client
}

func NewSystemHandler(db *gorm.DB, cfg *config.Config) *SystemHandler {
	return &SystemHandler{
		db:  db,
		cfg: cfg,
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

func (h *SystemHandler) Health(c *fiber.Ctx) error {
	dbStatus := "ok"
	statusCode := fiber.StatusOK

	sqlDB, err := h.db.DB()
	if err != nil {
		dbStatus = "error: " + err.Error()
		statusCode = fiber.StatusServiceUnavailable
	} else if err := sqlDB.Ping(); err != nil {
		dbStatus = "unreachable: " + err.Error()
		statusCode = fiber.StatusServiceUnavailable
	}

	overall := "ok"
	if statusCode != fiber.StatusOK {
		overall = "degraded"
	}

	return c.Status(statusCode).JSON(fiber.Map{
		"status":  overall,
		"service": "bastion",
		"version": Version,
		"time":    time.Now().UTC().Format(time.RFC3339),
		"uptime":  time.Since(startTime).String(),
		"db":      dbStatus,
	})
}

func (h *SystemHandler) Info(c *fiber.Ctx) error {
	var serverCount, cronCount, cmdCount, sessionCount int64
	h.db.Model(&struct{}{}).Table("servers").Count(&serverCount)
	h.db.Model(&struct{}{}).Table("cron_jobs").Count(&cronCount)
	h.db.Model(&struct{}{}).Table("command_histories").Count(&cmdCount)
	h.db.Model(&struct{}{}).Table("ssh_sessions").Count(&sessionCount)

	return c.JSON(fiber.Map{
		"version":           Version,
		"uptime":            time.Since(startTime).String(),
		"servers":           serverCount,
		"cron_jobs":         cronCount,
		"commands_executed": cmdCount,
		"ssh_sessions":      sessionCount,
	})
}

func (h *SystemHandler) DashboardOverview(c *fiber.Ctx) error {
	// ─── Server counts ──────────────────────────────────────────────────
	var serverTotal, serverOnline, serverOffline int64
	h.db.Table("servers").Where("deleted_at IS NULL").Count(&serverTotal)
	h.db.Table("servers").Where("deleted_at IS NULL AND status = ?", "online").Count(&serverOnline)
	h.db.Table("servers").Where("deleted_at IS NULL AND status = ?", "offline").Count(&serverOffline)

	// ─── Cron job counts ────────────────────────────────────────────────
	var cronTotal, cronActive int64
	h.db.Table("cron_jobs").Where("deleted_at IS NULL").Count(&cronTotal)
	h.db.Table("cron_jobs").Where("deleted_at IS NULL AND enabled = ?", true).Count(&cronActive)

	// ─── Recent commands (last 24h) ─────────────────────────────────────
	var recentCommands int64
	h.db.Table("command_histories").
		Where("executed_at > ?", time.Now().Add(-24*time.Hour)).
		Count(&recentCommands)

	// ─── AI conversations ───────────────────────────────────────────────
	var aiConversations int64
	h.db.Table("ai_conversations").Where("deleted_at IS NULL").Count(&aiConversations)

	// ─── Coolify apps (optional — best-effort) ──────────────────────────
	coolifyApps := 0
	if h.cfg.CoolifyAPIToken != "" {
		coolifyApps = h.fetchCoolifyAppCount()
	}

	// ─── Build response ─────────────────────────────────────────────────
	return c.JSON(fiber.Map{
		"servers": fiber.Map{
			"total":   serverTotal,
			"online":  serverOnline,
			"offline": serverOffline,
		},
		"containers": fiber.Map{
			"total":   serverTotal,
			"running": serverOnline,
		},
		"cron_jobs": fiber.Map{
			"total":  cronTotal,
			"active": cronActive,
		},
		"recent_commands":  recentCommands,
		"ai_conversations": aiConversations,
		"coolify": fiber.Map{
			"apps": coolifyApps,
		},
		"uptime_seconds": int64(time.Since(startTime).Seconds()),
	})
}

// fetchCoolifyAppCount calls the Coolify API to count deployed applications.
func (h *SystemHandler) fetchCoolifyAppCount() int {
	url := fmt.Sprintf("%s/api/v1/applications", h.cfg.CoolifyAPIURL)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		slog.Warn("Coolify request build failed", "error", err)
		return 0
	}
	req.Header.Set("Authorization", h.cfg.CoolifyAPIToken)
	req.Header.Set("Accept", "application/json")

	resp, err := h.client.Do(req)
	if err != nil {
		slog.Warn("Coolify API unreachable for dashboard", "error", err)
		return 0
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0
	}

	var apps []json.RawMessage
	if err := json.Unmarshal(body, &apps); err != nil {
		slog.Warn("Coolify apps parse failed", "error", err)
		return 0
	}
	return len(apps)
}
