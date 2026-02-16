package handlers

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/ahmetk3436/bastion/internal/models"
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
)

type DockerHandler struct {
	serverHandler *ServerHandler
}

func NewDockerHandler(serverHandler *ServerHandler) *DockerHandler {
	return &DockerHandler{serverHandler: serverHandler}
}

func (h *DockerHandler) execSSH(serverID uuid.UUID, command string) (string, error) {
	var server models.Server
	if err := h.serverHandler.GetDB().First(&server, "id = ?", serverID).Error; err != nil {
		return "", fmt.Errorf("server not found")
	}

	password, privateKey, err := h.serverHandler.GetDecryptedCredentials(&server)
	if err != nil {
		return "", fmt.Errorf("failed to decrypt credentials: %w", err)
	}

	client, err := h.serverHandler.GetSSHPool().GetConnection(server.Host, server.Port, server.Username, password, privateKey, server.AuthType)
	if err != nil {
		return "", fmt.Errorf("SSH connection failed: %w", err)
	}

	session, err := client.NewSession()
	if err != nil {
		return "", fmt.Errorf("SSH session failed: %w", err)
	}
	defer session.Close()

	output, err := session.CombinedOutput(command)
	return string(output), err
}

// sanitizeContainerID validates that a container ID only contains safe characters.
func sanitizeContainerID(id string) bool {
	for _, ch := range id {
		if !((ch >= 'a' && ch <= 'f') || (ch >= '0' && ch <= '9') || (ch >= 'A' && ch <= 'F') || ch == '-' || ch == '_' || ch == '.' || (ch >= 'g' && ch <= 'z') || (ch >= 'G' && ch <= 'Z')) {
			return false
		}
	}
	return len(id) > 0 && len(id) <= 128
}

// ListContainers returns all Docker containers.
func (h *DockerHandler) ListContainers(c *fiber.Ctx) error {
	serverID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error":   true,
			"message": "Invalid server ID",
		})
	}

	output, err := h.execSSH(serverID, `docker ps -a --format '{{json .}}'`)
	if err != nil {
		return c.Status(fiber.StatusBadGateway).JSON(fiber.Map{
			"error":   true,
			"message": "Failed to list containers: " + err.Error(),
		})
	}

	containers := parseDockerJSONLines(output)
	return c.JSON(fiber.Map{"containers": containers})
}

// ContainerAction performs start/stop/restart/rm on a container.
func (h *DockerHandler) ContainerAction(c *fiber.Ctx) error {
	serverID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error":   true,
			"message": "Invalid server ID",
		})
	}

	cid := c.Params("cid")
	if !sanitizeContainerID(cid) {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error":   true,
			"message": "Invalid container ID",
		})
	}

	var req struct {
		Action string `json:"action"`
	}
	if err := c.BodyParser(&req); err != nil || req.Action == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error":   true,
			"message": "Action is required (start, stop, restart, rm)",
		})
	}

	validActions := map[string]bool{
		"start": true, "stop": true, "restart": true, "rm": true,
	}
	if !validActions[req.Action] {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error":   true,
			"message": "Invalid action. Must be: start, stop, restart, rm",
		})
	}

	cmd := fmt.Sprintf("docker %s %s", req.Action, cid)
	output, err := h.execSSH(serverID, cmd)
	if err != nil {
		return c.Status(fiber.StatusBadGateway).JSON(fiber.Map{
			"error":   true,
			"message": "Container action failed: " + err.Error(),
			"output":  output,
		})
	}

	return c.JSON(fiber.Map{
		"message": fmt.Sprintf("Container %s: %s", cid, req.Action),
		"output":  strings.TrimSpace(output),
	})
}

// ContainerStats returns real-time stats for a container.
func (h *DockerHandler) ContainerStats(c *fiber.Ctx) error {
	serverID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error":   true,
			"message": "Invalid server ID",
		})
	}

	cid := c.Params("cid")
	if !sanitizeContainerID(cid) {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error":   true,
			"message": "Invalid container ID",
		})
	}

	cmd := fmt.Sprintf(`docker stats %s --no-stream --format '{{json .}}'`, cid)
	output, err := h.execSSH(serverID, cmd)
	if err != nil {
		return c.Status(fiber.StatusBadGateway).JSON(fiber.Map{
			"error":   true,
			"message": "Failed to get container stats: " + err.Error(),
		})
	}

	output = strings.TrimSpace(output)
	var stats map[string]interface{}
	if err := json.Unmarshal([]byte(output), &stats); err != nil {
		return c.JSON(fiber.Map{"stats": output})
	}

	return c.JSON(fiber.Map{"stats": stats})
}

// ContainerLogs returns recent logs from a container.
func (h *DockerHandler) ContainerLogs(c *fiber.Ctx) error {
	serverID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error":   true,
			"message": "Invalid server ID",
		})
	}

	cid := c.Params("cid")
	if !sanitizeContainerID(cid) {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error":   true,
			"message": "Invalid container ID",
		})
	}

	tail := c.Query("tail", "200")
	// Validate tail is numeric
	for _, ch := range tail {
		if ch < '0' || ch > '9' {
			tail = "200"
			break
		}
	}

	cmd := fmt.Sprintf("docker logs --tail %s %s 2>&1", tail, cid)
	output, err := h.execSSH(serverID, cmd)
	if err != nil {
		// Docker logs may exit non-zero but still have output
		if output == "" {
			return c.Status(fiber.StatusBadGateway).JSON(fiber.Map{
				"error":   true,
				"message": "Failed to get container logs: " + err.Error(),
			})
		}
	}

	return c.JSON(fiber.Map{"logs": output})
}

// ListImages returns all Docker images.
func (h *DockerHandler) ListImages(c *fiber.Ctx) error {
	serverID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error":   true,
			"message": "Invalid server ID",
		})
	}

	output, err := h.execSSH(serverID, `docker images --format '{{json .}}'`)
	if err != nil {
		return c.Status(fiber.StatusBadGateway).JSON(fiber.Map{
			"error":   true,
			"message": "Failed to list images: " + err.Error(),
		})
	}

	images := parseDockerJSONLines(output)
	return c.JSON(fiber.Map{"images": images})
}

// PullImage pulls a Docker image.
func (h *DockerHandler) PullImage(c *fiber.Ctx) error {
	serverID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error":   true,
			"message": "Invalid server ID",
		})
	}

	var req struct {
		Image string `json:"image"`
	}
	if err := c.BodyParser(&req); err != nil || req.Image == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error":   true,
			"message": "Image name is required",
		})
	}

	// Basic validation: no shell metacharacters
	for _, ch := range req.Image {
		if ch == ';' || ch == '&' || ch == '|' || ch == '$' || ch == '`' || ch == '\'' || ch == '"' || ch == '(' || ch == ')' || ch == '{' || ch == '}' || ch == '<' || ch == '>' {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error":   true,
				"message": "Invalid image name",
			})
		}
	}

	cmd := fmt.Sprintf("docker pull %s", req.Image)
	output, err := h.execSSH(serverID, cmd)
	if err != nil {
		return c.Status(fiber.StatusBadGateway).JSON(fiber.Map{
			"error":   true,
			"message": "Failed to pull image: " + err.Error(),
			"output":  output,
		})
	}

	return c.JSON(fiber.Map{
		"message": "Image pulled successfully",
		"output":  strings.TrimSpace(output),
	})
}

// PruneImages removes dangling Docker images.
func (h *DockerHandler) PruneImages(c *fiber.Ctx) error {
	serverID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error":   true,
			"message": "Invalid server ID",
		})
	}

	output, err := h.execSSH(serverID, "docker image prune -f")
	if err != nil {
		return c.Status(fiber.StatusBadGateway).JSON(fiber.Map{
			"error":   true,
			"message": "Failed to prune images: " + err.Error(),
			"output":  output,
		})
	}

	return c.JSON(fiber.Map{
		"message": "Images pruned successfully",
		"output":  strings.TrimSpace(output),
	})
}

// RemoveImage removes a Docker image.
func (h *DockerHandler) RemoveImage(c *fiber.Ctx) error {
	serverID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error":   true,
			"message": "Invalid server ID",
		})
	}

	iid := c.Params("iid")
	if !sanitizeContainerID(iid) {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error":   true,
			"message": "Invalid image ID",
		})
	}

	cmd := fmt.Sprintf("docker rmi %s", iid)
	output, err := h.execSSH(serverID, cmd)
	if err != nil {
		return c.Status(fiber.StatusBadGateway).JSON(fiber.Map{
			"error":   true,
			"message": "Failed to remove image: " + err.Error(),
			"output":  output,
		})
	}

	return c.JSON(fiber.Map{
		"message": "Image removed successfully",
		"output":  strings.TrimSpace(output),
	})
}

// parseDockerJSONLines parses newline-separated JSON objects from docker --format '{{json .}}'.
func parseDockerJSONLines(output string) []map[string]interface{} {
	lines := strings.Split(strings.TrimSpace(output), "\n")
	var results []map[string]interface{}

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		var obj map[string]interface{}
		if err := json.Unmarshal([]byte(line), &obj); err == nil {
			results = append(results, obj)
		}
	}

	return results
}
