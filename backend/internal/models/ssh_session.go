package models

import (
	"time"

	"github.com/google/uuid"
)

type SSHSession struct {
	ID               uuid.UUID  `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	ServerID         uuid.UUID  `gorm:"type:uuid;not null;index" json:"server_id"`
	Server           Server     `gorm:"foreignKey:ServerID" json:"-"`
	StartedAt        time.Time  `gorm:"not null" json:"started_at"`
	EndedAt          *time.Time `json:"ended_at"`
	DurationSeconds  int        `json:"duration_seconds"`
	CommandsExecuted int        `gorm:"default:0" json:"commands_executed"`
	BytesTransferred int64      `gorm:"default:0" json:"bytes_transferred"`
}
