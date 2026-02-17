package handlers

import (
	"log/slog"
	"strconv"

	"github.com/ahmetk3436/bastion/internal/models"
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type CronHandler struct {
	db            *gorm.DB
	serverHandler *ServerHandler
}

func NewCronHandler(db *gorm.DB, serverHandler *ServerHandler) *CronHandler {
	return &CronHandler{db: db, serverHandler: serverHandler}
}

func (h *CronHandler) ListCrons(c *fiber.Ctx) error {
	serverID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error":   true,
			"message": "Invalid server ID",
		})
	}

	var crons []models.CronJob
	h.db.Where("server_id = ?", serverID).Order("created_at DESC").Find(&crons)

	return c.JSON(fiber.Map{"crons": crons})
}

func (h *CronHandler) CreateCron(c *fiber.Ctx) error {
	serverID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error":   true,
			"message": "Invalid server ID",
		})
	}

	var req struct {
		Name                  string `json:"name"`
		Schedule              string `json:"schedule"`
		Command               string `json:"command"`
		NotificationOnFailure *bool  `json:"notification_on_failure"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error":   true,
			"message": "Invalid request body",
		})
	}

	if req.Name == "" || req.Schedule == "" || req.Command == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error":   true,
			"message": "Name, schedule, and command are required",
		})
	}

	cron := models.CronJob{
		ServerID:              serverID,
		Name:                  req.Name,
		Schedule:              req.Schedule,
		Command:               req.Command,
		Enabled:               true,
		NotificationOnFailure: true,
	}
	if req.NotificationOnFailure != nil {
		cron.NotificationOnFailure = *req.NotificationOnFailure
	}

	if err := h.db.Create(&cron).Error; err != nil {
		slog.Error("Failed to create cron job", "error", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error":   true,
			"message": "Failed to create cron job",
		})
	}

	return c.Status(fiber.StatusCreated).JSON(cron)
}

func (h *CronHandler) UpdateCron(c *fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error":   true,
			"message": "Invalid cron ID",
		})
	}

	var cron models.CronJob
	if err := h.db.First(&cron, "id = ?", id).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error":   true,
			"message": "Cron job not found",
		})
	}

	var req struct {
		Name                  *string `json:"name"`
		Schedule              *string `json:"schedule"`
		Command               *string `json:"command"`
		NotificationOnFailure *bool   `json:"notification_on_failure"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error":   true,
			"message": "Invalid request body",
		})
	}

	if req.Name != nil {
		cron.Name = *req.Name
	}
	if req.Schedule != nil {
		cron.Schedule = *req.Schedule
	}
	if req.Command != nil {
		cron.Command = *req.Command
	}
	if req.NotificationOnFailure != nil {
		cron.NotificationOnFailure = *req.NotificationOnFailure
	}

	h.db.Save(&cron)
	return c.JSON(cron)
}

func (h *CronHandler) DeleteCron(c *fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error":   true,
			"message": "Invalid cron ID",
		})
	}

	h.db.Delete(&models.CronJob{}, "id = ?", id)
	return c.JSON(fiber.Map{"message": "Cron job deleted"})
}

func (h *CronHandler) RunCron(c *fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error":   true,
			"message": "Invalid cron ID",
		})
	}

	var cron models.CronJob
	if err := h.db.First(&cron, "id = ?", id).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error":   true,
			"message": "Cron job not found",
		})
	}

	// Execute via command handler
	var server models.Server
	if err := h.db.First(&server, "id = ?", cron.ServerID).Error; err != nil {
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
			"message": "SSH connection failed",
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

	output, err := session.CombinedOutput(cron.Command)

	status := "success"
	errMsg := ""
	if err != nil {
		status = "failed"
		errMsg = err.Error()
	}

	now := c.Context().Time()
	h.db.Model(&cron).Updates(map[string]interface{}{
		"last_run_at": now,
		"last_status": status,
		"last_output": string(output),
		"last_error":  errMsg,
	})

	return c.JSON(fiber.Map{
		"status":  status,
		"output":  string(output),
		"error":   errMsg,
		"cron_id": id,
	})
}

func (h *CronHandler) ToggleCron(c *fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error":   true,
			"message": "Invalid cron ID",
		})
	}

	var cron models.CronJob
	if err := h.db.First(&cron, "id = ?", id).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error":   true,
			"message": "Cron job not found",
		})
	}

	cron.Enabled = !cron.Enabled
	h.db.Save(&cron)

	return c.JSON(fiber.Map{
		"message": "Cron job toggled",
		"enabled": cron.Enabled,
	})
}

func (h *CronHandler) GetCronLogs(c *fiber.Ctx) error {
	_ = strconv.Itoa(0) // suppress unused import
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error":   true,
			"message": "Invalid cron ID",
		})
	}

	var cron models.CronJob
	if err := h.db.First(&cron, "id = ?", id).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error":   true,
			"message": "Cron job not found",
		})
	}

	return c.JSON(fiber.Map{
		"cron_id":     cron.ID,
		"name":        cron.Name,
		"last_run_at": cron.LastRunAt,
		"last_status": cron.LastStatus,
		"last_output": cron.LastOutput,
		"last_error":  cron.LastError,
	})
}
