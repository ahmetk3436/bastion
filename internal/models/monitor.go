package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type Monitor struct {
	ID               uuid.UUID      `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	Name             string         `gorm:"not null" json:"name"`
	URL              string         `gorm:"not null" json:"url"`
	Type             string         `gorm:"default:'http'" json:"type"` // http, tcp, ping
	Method           string         `gorm:"default:'GET'" json:"method"`
	IntervalSeconds  int            `gorm:"default:60" json:"interval_seconds"`
	TimeoutMs        int            `gorm:"default:5000" json:"timeout_ms"`
	ExpectedStatus   int            `gorm:"default:200" json:"expected_status"`
	Enabled          bool           `gorm:"default:true" json:"enabled"`
	LastCheckedAt    *time.Time     `json:"last_checked_at"`
	LastStatus       string         `gorm:"default:'unknown'" json:"last_status"` // up, down, unknown
	LastResponseMs   int            `json:"last_response_ms"`
	ConsecutiveFails int            `gorm:"default:0" json:"consecutive_fails"`
	UptimePercent    float64        `gorm:"default:100" json:"uptime_percent"`
	CreatedAt        time.Time      `json:"created_at"`
	UpdatedAt        time.Time      `json:"updated_at"`
	DeletedAt        gorm.DeletedAt `gorm:"index" json:"-"`
}

type MonitorPing struct {
	ID         uuid.UUID `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	MonitorID  uuid.UUID `gorm:"type:uuid;not null;index" json:"monitor_id"`
	Status     string    `gorm:"not null" json:"status"` // up, down
	ResponseMs int       `json:"response_ms"`
	StatusCode int       `json:"status_code"`
	Error      string    `json:"error"`
	CheckedAt  time.Time `gorm:"not null" json:"checked_at"`
}

type SSLCert struct {
	ID            uuid.UUID  `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	Domain        string     `gorm:"not null;uniqueIndex" json:"domain"`
	Issuer        string     `json:"issuer"`
	ValidFrom     time.Time  `json:"valid_from"`
	ValidTo       time.Time  `json:"valid_to"`
	DaysRemaining int        `json:"days_remaining"`
	LastCheckedAt *time.Time `json:"last_checked_at"`
	CreatedAt     time.Time  `json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
}
