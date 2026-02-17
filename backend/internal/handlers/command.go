package handlers

import (
	"bytes"
	"strconv"
	"time"

	"github.com/ahmetk3436/bastion/internal/models"
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"golang.org/x/crypto/ssh"
)

type CommandHandler struct {
	serverHandler *ServerHandler
}

func NewCommandHandler(serverHandler *ServerHandler) *CommandHandler {
	return &CommandHandler{serverHandler: serverHandler}
}

func (h *CommandHandler) ExecCommand(c *fiber.Ctx) error {
	serverID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error":   true,
			"message": "Invalid server ID",
		})
	}

	var req struct {
		Command string `json:"command"`
	}
	if err := c.BodyParser(&req); err != nil || req.Command == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error":   true,
			"message": "Command is required",
		})
	}

	db := h.serverHandler.GetDB()

	var server models.Server
	if err := db.First(&server, "id = ?", serverID).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error":   true,
			"message": "Server not found",
		})
	}

	password, privateKey, err := h.serverHandler.GetDecryptedCredentials(&server)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error":   true,
			"message": "Failed to decrypt credentials",
		})
	}

	pool := h.serverHandler.GetSSHPool()
	client, err := pool.GetConnection(server.Host, server.Port, server.Username, password, privateKey, server.AuthType)
	if err != nil {
		return c.Status(fiber.StatusBadGateway).JSON(fiber.Map{
			"error":   true,
			"message": "SSH connection failed: " + err.Error(),
		})
	}

	session, err := client.NewSession()
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error":   true,
			"message": "Failed to create SSH session",
		})
	}
	defer session.Close()

	start := time.Now()

	var stdout, stderr bytes.Buffer
	session.Stdout = &stdout
	session.Stderr = &stderr

	exitCode := 0
	if err := session.Run(req.Command); err != nil {
		if exitErr, ok := err.(*ssh.ExitError); ok {
			exitCode = exitErr.ExitStatus()
		} else {
			exitCode = -1
		}
	}

	duration := time.Since(start)
	output := stdout.String()
	if stderr.Len() > 0 {
		if output != "" {
			output += "\n"
		}
		output += stderr.String()
	}

	// Save to history
	history := models.CommandHistory{
		ServerID:   serverID,
		Command:    req.Command,
		Output:     output,
		ExitCode:   exitCode,
		ExecutedAt: start,
		DurationMs: int(duration.Milliseconds()),
	}
	db.Create(&history)

	return c.JSON(fiber.Map{
		"command":     req.Command,
		"output":      output,
		"exit_code":   exitCode,
		"duration_ms": duration.Milliseconds(),
		"id":          history.ID,
	})
}

// ExitError wraps ssh exit status
type ExitError interface {
	ExitStatus() int
}

func (h *CommandHandler) GetHistory(c *fiber.Ctx) error {
	serverID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error":   true,
			"message": "Invalid server ID",
		})
	}

	page, _ := strconv.Atoi(c.Query("page", "1"))
	perPage, _ := strconv.Atoi(c.Query("per_page", "50"))
	if page < 1 {
		page = 1
	}
	if perPage < 1 || perPage > 100 {
		perPage = 50
	}

	db := h.serverHandler.GetDB()
	var total int64
	db.Model(&models.CommandHistory{}).Where("server_id = ?", serverID).Count(&total)

	var history []models.CommandHistory
	db.Where("server_id = ?", serverID).
		Order("executed_at DESC").
		Offset((page - 1) * perPage).
		Limit(perPage).
		Find(&history)

	return c.JSON(fiber.Map{
		"history":  history,
		"total":    total,
		"page":     page,
		"per_page": perPage,
	})
}

func (h *CommandHandler) ListFavorites(c *fiber.Ctx) error {
	db := h.serverHandler.GetDB()
	var favorites []models.CommandHistory
	db.Where("is_favorite = ?", true).Order("executed_at DESC").Find(&favorites)
	return c.JSON(fiber.Map{"favorites": favorites})
}

func (h *CommandHandler) ToggleFavorite(c *fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error":   true,
			"message": "Invalid command ID",
		})
	}

	db := h.serverHandler.GetDB()
	var cmd models.CommandHistory
	if err := db.First(&cmd, "id = ?", id).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error":   true,
			"message": "Command not found",
		})
	}

	cmd.IsFavorite = !cmd.IsFavorite
	db.Save(&cmd)

	return c.JSON(fiber.Map{
		"message":     "Favorite toggled",
		"is_favorite": cmd.IsFavorite,
	})
}

func (h *CommandHandler) DeleteFavorite(c *fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error":   true,
			"message": "Invalid command ID",
		})
	}

	db := h.serverHandler.GetDB()
	db.Model(&models.CommandHistory{}).Where("id = ?", id).Update("is_favorite", false)

	return c.JSON(fiber.Map{"message": "Favorite removed"})
}
