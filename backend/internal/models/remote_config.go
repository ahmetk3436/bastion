package models

import (
	"time"

	"github.com/google/uuid"
)

type RemoteConfig struct {
	ID        uuid.UUID `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	Key       string    `gorm:"uniqueIndex;not null" json:"key"`
	Value     string    `gorm:"not null" json:"value"`
	Type      string    `gorm:"default:'string'" json:"type"` // string, bool, int, json
	UpdatedAt time.Time `json:"updated_at"`
}
