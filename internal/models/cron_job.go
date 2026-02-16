package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type CronJob struct {
	ID                    uuid.UUID      `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	ServerID              uuid.UUID      `gorm:"type:uuid;not null;index" json:"server_id"`
	Server                Server         `gorm:"foreignKey:ServerID" json:"-"`
	Name                  string         `gorm:"not null" json:"name"`
	Schedule              string         `gorm:"not null" json:"schedule"` // cron expression
	Command               string         `gorm:"not null" json:"command"`
	Enabled               bool           `gorm:"default:true" json:"enabled"`
	LastRunAt             *time.Time     `json:"last_run_at"`
	LastStatus            string         `gorm:"default:''" json:"last_status"` // success, failed, running
	LastOutput            string         `gorm:"type:text" json:"last_output"`
	LastError             string         `gorm:"type:text" json:"last_error"`
	NextRunAt             *time.Time     `json:"next_run_at"`
	NotificationOnFailure bool           `gorm:"default:true" json:"notification_on_failure"`
	CreatedAt             time.Time      `json:"created_at"`
	UpdatedAt             time.Time      `json:"updated_at"`
	DeletedAt             gorm.DeletedAt `gorm:"index" json:"-"`
}
