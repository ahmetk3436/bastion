package services

import (
	"encoding/json"
	"strings"
	"time"

	"github.com/ahmetk3436/bastion/internal/models"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// ContextService provides enriched system context for AI
type ContextService struct {
	db *gorm.DB
}

// NewContextService creates a new context service
func NewContextService(db *gorm.DB) *ContextService {
	return &ContextService{db: db}
}

// SystemContext is the full system context structure
type SystemContext struct {
	Timestamp      time.Time              `json:"timestamp"`
	Server         *ServerContext         `json:"server,omitempty"`
	Metrics        *models.ServerMetrics  `json:"metrics,omitempty"`
	Monitors       []MonitorStatus        `json:"monitors,omitempty"`
	Alerts         []AlertStatus          `json:"alerts,omitempty"`
	RecentCommands []CommandSummary       `json:"recent_commands,omitempty"`
	CoolifyApps    []CoolifyApp           `json:"coolify_apps,omitempty"`
}

// ServerContext contains server information
type ServerContext struct {
	ID              uuid.UUID  `json:"id"`
	Name            string     `json:"name"`
	Host            string     `json:"host"`
	Port            int        `json:"port"`
	Status          string     `json:"status"`
	LastConnectedAt *time.Time `json:"last_connected_at,omitempty"`
}

// MonitorStatus represents monitor status in context
type MonitorStatus struct {
	ID             uuid.UUID `json:"id"`
	Name           string    `json:"name"`
	URL            string    `json:"url"`
	Type           string    `json:"type"`
	LastStatus     string    `json:"last_status"`
	LastResponseMs int       `json:"last_response_ms"`
	UptimePercent  float64   `json:"uptime_percent"`
	Enabled        bool      `json:"enabled"`
}

// AlertStatus represents alert status in context
type AlertStatus struct {
	ID        uuid.UUID `json:"id"`
	RuleID    uuid.UUID `json:"rule_id"`
	Severity  string    `json:"severity"`
	Message   string    `json:"message"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
}

// CommandSummary represents a command history summary
type CommandSummary struct {
	Command    string    `json:"command"`
	ExitCode   int       `json:"exit_code"`
	ExecutedAt time.Time `json:"executed_at"`
	DurationMs int       `json:"duration_ms"`
}

// CoolifyApp represents a Coolify application
type CoolifyApp struct {
	UUID   string `json:"uuid"`
	Name   string `json:"name"`
	Status string `json:"status"`
}

// GetFullContext retrieves complete system context for a server
func (s *ContextService) GetFullContext(serverID uuid.UUID) *SystemContext {
	ctx := &SystemContext{
		Timestamp: time.Now(),
	}

	// Get server info
	var server models.Server
	if err := s.db.First(&server, "id = ?", serverID).Error; err == nil {
		ctx.Server = &ServerContext{
			ID:              server.ID,
			Name:            server.Name,
			Host:            server.Host,
			Port:            server.Port,
			Status:          server.Status,
			LastConnectedAt: server.LastConnectedAt,
		}
	}

	// Get latest metrics
	var metrics models.ServerMetrics
	if err := s.db.Where("server_id = ?", serverID).
		Order("collected_at DESC").
		First(&metrics).Error; err == nil {
		ctx.Metrics = &metrics
	}

	// Get active monitors
	var monitors []models.Monitor
	if err := s.db.Where("enabled = ?", true).
		Order("uptime_percent ASC").
		Find(&monitors).Error; err == nil {
		ctx.Monitors = make([]MonitorStatus, len(monitors))
		for i, m := range monitors {
			ctx.Monitors[i] = MonitorStatus{
				ID:             m.ID,
				Name:           m.Name,
				URL:            m.URL,
				Type:           m.Type,
				LastStatus:     m.LastStatus,
				LastResponseMs: m.LastResponseMs,
				UptimePercent:  m.UptimePercent,
				Enabled:        m.Enabled,
			}
		}
	}

	// Get firing alerts
	var alerts []models.Alert
	if err := s.db.Where("status = ?", "firing").
		Order("created_at DESC").
		Limit(20).
		Find(&alerts).Error; err == nil {
		ctx.Alerts = make([]AlertStatus, len(alerts))
		for i, a := range alerts {
			ctx.Alerts[i] = AlertStatus{
				ID:        a.ID,
				RuleID:    a.RuleID,
				Severity:  a.Severity,
				Message:   a.Message,
				Status:    a.Status,
				CreatedAt: a.CreatedAt,
			}
		}
	}

	// Get recent commands for this server
	var commands []models.CommandHistory
	if err := s.db.Where("server_id = ?", serverID).
		Order("executed_at DESC").
		Limit(10).
		Find(&commands).Error; err == nil {
		ctx.RecentCommands = make([]CommandSummary, len(commands))
		for i, c := range commands {
			ctx.RecentCommands[i] = CommandSummary{
				Command:    c.Command,
				ExitCode:   c.ExitCode,
				ExecutedAt: c.ExecutedAt,
				DurationMs: c.DurationMs,
			}
		}
	}

	return ctx
}

// GetAllServersContext returns context for all servers
func (s *ContextService) GetAllServersContext() []ServerContext {
	var servers []models.Server
	if err := s.db.Order("name ASC").Find(&servers).Error; err != nil {
		return nil
	}

	result := make([]ServerContext, len(servers))
	for i, s := range servers {
		result[i] = ServerContext{
			ID:              s.ID,
			Name:            s.Name,
			Host:            s.Host,
			Port:            s.Port,
			Status:          s.Status,
			LastConnectedAt: s.LastConnectedAt,
		}
	}
	return result
}

// GetFiringAlertsCount returns count of firing alerts by severity
func (s *ContextService) GetFiringAlertsCount() map[string]int64 {
	var result struct {
		Critical int64
		Warning  int64
		Info     int64
	}

	s.db.Model(&models.Alert{}).
		Where("status = ? AND severity = ?", "firing", "critical").
		Count(&result.Critical)

	s.db.Model(&models.Alert{}).
		Where("status = ? AND severity = ?", "firing", "warning").
		Count(&result.Warning)

	s.db.Model(&models.Alert{}).
		Where("status = ? AND severity = ?", "firing", "info").
		Count(&result.Info)

	return map[string]int64{
		"critical": result.Critical,
		"warning":  result.Warning,
		"info":     result.Info,
	}
}

// ToJSON converts context to formatted JSON string
func (c *SystemContext) ToJSON() string {
	data, _ := json.MarshalIndent(c, "", "  ")
	return string(data)
}

// ToPromptFormat converts context to prompt-friendly format
func (c *SystemContext) ToPromptFormat() string {
	var sb strings.Builder

	sb.WriteString("## Current System Context\n")
	sb.WriteString("```json\n")
	sb.WriteString(c.ToJSON())
	sb.WriteString("\n```\n")

	return sb.String()
}

// SetCoolifyApps sets Coolify apps in the context
func (c *SystemContext) SetCoolifyApps(apps []CoolifyApp) {
	c.CoolifyApps = apps
}
