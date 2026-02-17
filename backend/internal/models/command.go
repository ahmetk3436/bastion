package models

import (
	"time"

	"github.com/google/uuid"
)

type CommandHistory struct {
	ID         uuid.UUID `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	ServerID   uuid.UUID `gorm:"type:uuid;not null;index" json:"server_id"`
	Server     Server    `gorm:"foreignKey:ServerID" json:"-"`
	Command    string    `gorm:"not null" json:"command"`
	Output     string    `gorm:"type:text" json:"output"`
	ExitCode   int       `json:"exit_code"`
	ExecutedAt time.Time `gorm:"not null" json:"executed_at"`
	DurationMs int       `json:"duration_ms"`
	IsFavorite bool      `gorm:"default:false" json:"is_favorite"`
}
