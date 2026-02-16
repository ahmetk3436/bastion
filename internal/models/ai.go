package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

type AIConversation struct {
	ID        uuid.UUID      `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	Title     string         `gorm:"not null" json:"title"`
	Messages  datatypes.JSON `gorm:"type:jsonb;default:'[]'" json:"messages"`
	Context   string         `gorm:"type:text" json:"context"`
	ServerID  *uuid.UUID     `gorm:"type:uuid" json:"server_id"`
	Server    *Server        `gorm:"foreignKey:ServerID" json:"-"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
}
