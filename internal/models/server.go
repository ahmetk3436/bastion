package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type Server struct {
	ID                  uuid.UUID      `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	Name                string         `gorm:"not null" json:"name"`
	Host                string         `gorm:"not null" json:"host"`
	Port                int            `gorm:"default:22" json:"port"`
	Username            string         `gorm:"not null" json:"username"`
	AuthType            string         `gorm:"not null;default:'password'" json:"auth_type"` // password or key
	EncryptedPassword   string         `gorm:"" json:"-"`
	EncryptedPrivateKey string         `gorm:"type:text" json:"-"`
	Fingerprint         string         `gorm:"" json:"fingerprint"`
	IsDefault           bool           `gorm:"default:false" json:"is_default"`
	Status              string         `gorm:"default:'unknown'" json:"status"` // online, offline, unknown
	LastConnectedAt     *time.Time     `json:"last_connected_at"`
	CreatedAt           time.Time      `json:"created_at"`
	UpdatedAt           time.Time      `json:"updated_at"`
	DeletedAt           gorm.DeletedAt `gorm:"index" json:"-"`
}
