package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/datatypes"
)

type AuditLog struct {
	ID        uuid.UUID      `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	Actor     string         `gorm:"not null" json:"actor"`
	Action    string         `gorm:"not null" json:"action"` // restart, deploy, kill, execute, acknowledge, etc.
	Target    string         `json:"target"`
	Details   datatypes.JSON `gorm:"type:jsonb" json:"details"`
	CreatedAt time.Time      `json:"created_at"`
}
