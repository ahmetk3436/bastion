package config

import (
	"os"
	"strconv"
)

type Config struct {
	// Server
	Port string

	// Database
	DBHost     string
	DBPort     string
	DBUser     string
	DBPassword string
	DBName     string
	DBSSLMode  string

	// Auth (single user)
	AdminUsername    string
	AdminPassword   string // bcrypt hash stored, plaintext in env for initial setup
	AdminDisplayName string
	AdminRole       string
	JWTSecret       string

	// SSH Encryption
	SSHEncryptionKey string // 32-byte hex for AES-256-GCM

	// Coolify
	CoolifyAPIURL   string
	CoolifyAPIToken string

	// Ops Backend
	OpsBackendURL string
	OpsAdminToken string

	// AI (GLM-5)
	GLMAPIKey string
	GLMAPIURL string
	GLMModel  string

	// Web Search
	TavilyAPIKey string
	SerperAPIKey string

	// Metrics
	MetricsCollectInterval int // seconds
}

func Load() *Config {
	metricsInterval, _ := strconv.Atoi(getEnv("METRICS_COLLECT_INTERVAL", "60"))
	return &Config{
		Port:                   getEnv("PORT", "8097"),
		DBHost:                 getEnv("DB_HOST", "localhost"),
		DBPort:                 getEnv("DB_PORT", "5432"),
		DBUser:                 getEnv("DB_USER", "postgres"),
		DBPassword:             getEnv("DB_PASSWORD", ""),
		DBName:                 getEnv("DB_NAME", "bastion_db"),
		DBSSLMode:              getEnv("DB_SSLMODE", "disable"),
		AdminUsername:          getEnv("ADMIN_USERNAME", "ahmet"),
		AdminPassword:          getEnv("ADMIN_PASSWORD", ""),
		AdminDisplayName:       getEnv("ADMIN_DISPLAY_NAME", "Ahmet"),
		AdminRole:              getEnv("ADMIN_ROLE", "admin"),
		JWTSecret:              getEnv("JWT_SECRET", ""),
		SSHEncryptionKey:       getEnv("SSH_ENCRYPTION_KEY", ""),
		CoolifyAPIURL:         getEnv("COOLIFY_API_URL", "http://89.47.113.196:8000"),
		CoolifyAPIToken:       getEnv("COOLIFY_API_TOKEN", ""),
		OpsBackendURL:         getEnv("OPS_BACKEND_URL", "http://89.47.113.196:8095"),
		OpsAdminToken:         getEnv("OPS_ADMIN_TOKEN", ""),
		GLMAPIKey:             getEnv("GLM_API_KEY", ""),
		GLMAPIURL:             getEnv("GLM_API_URL", "https://api.z.ai/api/paas/v4/chat/completions"),
		GLMModel:              getEnv("GLM_MODEL", "glm-5"),
		TavilyAPIKey:          getEnv("TAVILY_API_KEY", ""),
		SerperAPIKey:          getEnv("SERPER_API_KEY", ""),
		MetricsCollectInterval: metricsInterval,
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
