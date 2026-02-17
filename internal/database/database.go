package database

import (
	"fmt"
	"log/slog"

	"github.com/ahmetk3436/bastion/internal/config"
	"github.com/ahmetk3436/bastion/internal/models"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

var DB *gorm.DB

func Connect(cfg *config.Config) error {
	dsn := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=%s",
		cfg.DBHost, cfg.DBPort, cfg.DBUser, cfg.DBPassword, cfg.DBName, cfg.DBSSLMode)

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}

	DB = db
	slog.Info("Database connected", "host", cfg.DBHost, "db", cfg.DBName)
	return nil
}

func Migrate() error {
	return DB.AutoMigrate(
		&models.Server{},
		&models.SSHSession{},
		&models.CronJob{},
		&models.CommandHistory{},
		&models.ServerMetrics{},
		&models.AIConversation{},
		&models.Monitor{},
		&models.MonitorPing{},
		&models.SSLCert{},
		&models.AlertRule{},
		&models.Alert{},
		&models.AuditLog{},
		&models.RemoteConfig{},
	)
}
