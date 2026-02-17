package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type AlertRule struct {
	ID                  uuid.UUID      `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	Name                string         `gorm:"not null" json:"name"`
	Type                string         `gorm:"not null" json:"type"` // cpu, memory, disk, response_time, uptime
	Metric              string         `gorm:"not null" json:"metric"`
	Operator            string         `gorm:"not null;default:'>'" json:"operator"` // >, <, >=, <=, ==
	Threshold           float64        `gorm:"not null" json:"threshold"`
	DurationSeconds     int            `gorm:"default:60" json:"duration_seconds"`
	NotificationChannel string         `gorm:"default:'dashboard'" json:"notification_channel"` // dashboard, email
	Enabled             bool           `gorm:"default:true" json:"enabled"`
	LastTriggeredAt     *time.Time     `json:"last_triggered_at"`
	CreatedAt           time.Time      `json:"created_at"`
	UpdatedAt           time.Time      `json:"updated_at"`
	DeletedAt           gorm.DeletedAt `gorm:"index" json:"-"`
}

type Alert struct {
	ID             uuid.UUID  `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	RuleID         uuid.UUID  `gorm:"type:uuid;not null;index" json:"rule_id"`
	Severity       string     `gorm:"not null;default:'warning'" json:"severity"` // critical, warning, info
	Message        string     `gorm:"not null" json:"message"`
	Status         string     `gorm:"not null;default:'firing'" json:"status"` // firing, acknowledged, resolved
	AcknowledgedAt *time.Time `json:"acknowledged_at"`
	ResolvedAt     *time.Time `json:"resolved_at"`
	CreatedAt      time.Time  `json:"created_at"`
}
