package handlers

import (
	"log/slog"
	"time"

	"github.com/ahmetk3436/bastion/internal/crypto"
	"github.com/ahmetk3436/bastion/internal/models"
	"github.com/ahmetk3436/bastion/internal/services"
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type ServerHandler struct {
	db        *gorm.DB
	encryptor *crypto.Encryptor
	sshPool   *services.SSHPool
}

func NewServerHandler(db *gorm.DB, encryptor *crypto.Encryptor, sshPool *services.SSHPool) *ServerHandler {
	return &ServerHandler{db: db, encryptor: encryptor, sshPool: sshPool}
}

func (h *ServerHandler) ListServers(c *fiber.Ctx) error {
	var servers []models.Server
	if err := h.db.Order("created_at DESC").Find(&servers).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error":   true,
			"message": "Failed to list servers",
		})
	}
	return c.JSON(fiber.Map{"servers": servers})
}

func (h *ServerHandler) CreateServer(c *fiber.Ctx) error {
	var req struct {
		Name       string `json:"name"`
		Host       string `json:"host"`
		Port       int    `json:"port"`
		Username   string `json:"username"`
		AuthType   string `json:"auth_type"`
		Password   string `json:"password"`
		PrivateKey string `json:"private_key"`
		IsDefault  bool   `json:"is_default"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error":   true,
			"message": "Invalid request body",
		})
	}

	if req.Name == "" || req.Host == "" || req.Username == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error":   true,
			"message": "Name, host, and username are required",
		})
	}

	if req.Port == 0 {
		req.Port = 22
	}
	if req.AuthType == "" {
		req.AuthType = "password"
	}

	// Test connection first
	fingerprint, err := services.TestSSHConnection(req.Host, req.Port, req.Username, req.Password, req.PrivateKey, req.AuthType)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error":   true,
			"message": "SSH connection test failed: " + err.Error(),
		})
	}

	// Encrypt credentials
	server := models.Server{
		Name:        req.Name,
		Host:        req.Host,
		Port:        req.Port,
		Username:    req.Username,
		AuthType:    req.AuthType,
		Fingerprint: fingerprint,
		IsDefault:   req.IsDefault,
		Status:      "online",
	}

	now := time.Now()
	server.LastConnectedAt = &now

	if req.AuthType == "key" && req.PrivateKey != "" {
		encrypted, err := h.encryptor.Encrypt(req.PrivateKey)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error":   true,
				"message": "Failed to encrypt private key",
			})
		}
		server.EncryptedPrivateKey = encrypted
	} else if req.Password != "" {
		encrypted, err := h.encryptor.Encrypt(req.Password)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error":   true,
				"message": "Failed to encrypt password",
			})
		}
		server.EncryptedPassword = encrypted
	}

	// If this is default, unset other defaults
	if req.IsDefault {
		h.db.Model(&models.Server{}).Where("is_default = ?", true).Update("is_default", false)
	}

	if err := h.db.Create(&server).Error; err != nil {
		slog.Error("Failed to create server", "error", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error":   true,
			"message": "Failed to create server",
		})
	}

	return c.Status(fiber.StatusCreated).JSON(server)
}

func (h *ServerHandler) GetServer(c *fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error":   true,
			"message": "Invalid server ID",
		})
	}

	var server models.Server
	if err := h.db.First(&server, "id = ?", id).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error":   true,
			"message": "Server not found",
		})
	}

	// Get latest metrics
	var latestMetrics models.ServerMetrics
	h.db.Where("server_id = ?", id).Order("collected_at DESC").First(&latestMetrics)

	return c.JSON(fiber.Map{
		"server":  server,
		"metrics": latestMetrics,
	})
}

func (h *ServerHandler) UpdateServer(c *fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error":   true,
			"message": "Invalid server ID",
		})
	}

	var server models.Server
	if err := h.db.First(&server, "id = ?", id).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error":   true,
			"message": "Server not found",
		})
	}

	var req struct {
		Name       *string `json:"name"`
		Host       *string `json:"host"`
		Port       *int    `json:"port"`
		Username   *string `json:"username"`
		AuthType   *string `json:"auth_type"`
		Password   *string `json:"password"`
		PrivateKey *string `json:"private_key"`
		IsDefault  *bool   `json:"is_default"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error":   true,
			"message": "Invalid request body",
		})
	}

	if req.Name != nil {
		server.Name = *req.Name
	}
	if req.Host != nil {
		server.Host = *req.Host
	}
	if req.Port != nil {
		server.Port = *req.Port
	}
	if req.Username != nil {
		server.Username = *req.Username
	}
	if req.AuthType != nil {
		server.AuthType = *req.AuthType
	}
	if req.Password != nil && *req.Password != "" {
		encrypted, err := h.encryptor.Encrypt(*req.Password)
		if err == nil {
			server.EncryptedPassword = encrypted
		}
	}
	if req.PrivateKey != nil && *req.PrivateKey != "" {
		encrypted, err := h.encryptor.Encrypt(*req.PrivateKey)
		if err == nil {
			server.EncryptedPrivateKey = encrypted
		}
	}
	if req.IsDefault != nil && *req.IsDefault {
		h.db.Model(&models.Server{}).Where("is_default = ?", true).Update("is_default", false)
		server.IsDefault = true
	}

	if err := h.db.Save(&server).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error":   true,
			"message": "Failed to update server",
		})
	}

	return c.JSON(server)
}

func (h *ServerHandler) DeleteServer(c *fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error":   true,
			"message": "Invalid server ID",
		})
	}

	if err := h.db.Delete(&models.Server{}, "id = ?", id).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error":   true,
			"message": "Failed to delete server",
		})
	}

	return c.JSON(fiber.Map{"message": "Server deleted"})
}

func (h *ServerHandler) TestConnection(c *fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error":   true,
			"message": "Invalid server ID",
		})
	}

	var server models.Server
	if err := h.db.First(&server, "id = ?", id).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error":   true,
			"message": "Server not found",
		})
	}

	password, privateKey, err := h.decryptCredentials(&server)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error":   true,
			"message": "Failed to decrypt credentials",
		})
	}

	fingerprint, err := services.TestSSHConnection(server.Host, server.Port, server.Username, password, privateKey, server.AuthType)
	if err != nil {
		h.db.Model(&server).Updates(map[string]interface{}{"status": "offline"})
		return c.Status(fiber.StatusBadGateway).JSON(fiber.Map{
			"error":       true,
			"message":     "Connection failed: " + err.Error(),
			"fingerprint": fingerprint,
		})
	}

	now := time.Now()
	h.db.Model(&server).Updates(map[string]interface{}{
		"status":            "online",
		"fingerprint":       fingerprint,
		"last_connected_at": now,
	})

	return c.JSON(fiber.Map{
		"message":     "Connection successful",
		"fingerprint": fingerprint,
	})
}

func (h *ServerHandler) GetMetrics(c *fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error":   true,
			"message": "Invalid server ID",
		})
	}

	period := c.Query("period", "1h")
	var since time.Time
	switch period {
	case "24h":
		since = time.Now().Add(-24 * time.Hour)
	case "7d":
		since = time.Now().Add(-7 * 24 * time.Hour)
	default:
		since = time.Now().Add(-1 * time.Hour)
	}

	var metrics []models.ServerMetrics
	h.db.Where("server_id = ? AND collected_at >= ?", id, since).
		Order("collected_at ASC").
		Find(&metrics)

	return c.JSON(fiber.Map{"metrics": metrics, "period": period})
}

func (h *ServerHandler) GetLiveMetrics(c *fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error":   true,
			"message": "Invalid server ID",
		})
	}

	var metrics models.ServerMetrics
	if err := h.db.Where("server_id = ?", id).Order("collected_at DESC").First(&metrics).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error":   true,
			"message": "No metrics available",
		})
	}

	return c.JSON(metrics)
}

func (h *ServerHandler) decryptCredentials(server *models.Server) (password, privateKey string, err error) {
	if server.EncryptedPassword != "" {
		password, err = h.encryptor.Decrypt(server.EncryptedPassword)
		if err != nil {
			return "", "", err
		}
	}
	if server.EncryptedPrivateKey != "" {
		privateKey, err = h.encryptor.Decrypt(server.EncryptedPrivateKey)
		if err != nil {
			return "", "", err
		}
	}
	return password, privateKey, nil
}

// GetDecryptedCredentials is used by other handlers that need SSH access
func (h *ServerHandler) GetDecryptedCredentials(server *models.Server) (password, privateKey string, err error) {
	return h.decryptCredentials(server)
}

// GetSSHPool returns the SSH connection pool
func (h *ServerHandler) GetSSHPool() *services.SSHPool {
	return h.sshPool
}

// GetDB returns the database connection
func (h *ServerHandler) GetDB() *gorm.DB {
	return h.db
}
