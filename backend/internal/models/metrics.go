package models

import (
	"time"

	"github.com/google/uuid"
)

type ServerMetrics struct {
	ID               uuid.UUID `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	ServerID         uuid.UUID `gorm:"type:uuid;not null;index" json:"server_id"`
	Server           Server    `gorm:"foreignKey:ServerID" json:"-"`
	CPUPercent       float64   `json:"cpu_percent"`
	MemoryUsedMB     float64   `json:"memory_used_mb"`
	MemoryTotalMB    float64   `json:"memory_total_mb"`
	DiskUsedGB       float64   `json:"disk_used_gb"`
	DiskTotalGB      float64   `json:"disk_total_gb"`
	NetworkRxBytes   int64     `json:"network_rx_bytes"`
	NetworkTxBytes   int64     `json:"network_tx_bytes"`
	ContainerCount   int       `json:"container_count"`
	ContainerRunning int       `json:"container_running"`
	LoadAvg1m        float64   `json:"load_avg_1m"`
	LoadAvg5m        float64   `json:"load_avg_5m"`
	LoadAvg15m       float64   `json:"load_avg_15m"`
	UptimeSeconds    int64     `json:"uptime_seconds"`
	CollectedAt      time.Time `gorm:"not null;index" json:"collected_at"`
}
