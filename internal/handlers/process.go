package handlers

import (
	"fmt"
	"strings"

	"github.com/ahmetk3436/bastion/internal/models"
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
)

type ProcessHandler struct {
	serverHandler *ServerHandler
}

func NewProcessHandler(serverHandler *ServerHandler) *ProcessHandler {
	return &ProcessHandler{serverHandler: serverHandler}
}

func (h *ProcessHandler) execSSH(serverID uuid.UUID, command string) (string, error) {
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

// ListProcesses returns the top 50 processes sorted by CPU usage.
func (h *ProcessHandler) ListProcesses(c *fiber.Ctx) error {
	serverID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error":   true,
			"message": "Invalid server ID",
		})
	}

	output, err := h.execSSH(serverID, "ps aux --sort=-%cpu | head -50")
	if err != nil {
		return c.Status(fiber.StatusBadGateway).JSON(fiber.Map{
			"error":   true,
			"message": "Failed to list processes: " + err.Error(),
		})
	}

	processes := parseProcesses(output)
	return c.JSON(fiber.Map{"processes": processes})
}

// KillProcess sends a signal to a process on the server.
func (h *ProcessHandler) KillProcess(c *fiber.Ctx) error {
	serverID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error":   true,
			"message": "Invalid server ID",
		})
	}

	pid := c.Params("pid")
	if pid == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error":   true,
			"message": "PID is required",
		})
	}

	var req struct {
		Signal string `json:"signal"`
	}
	if err := c.BodyParser(&req); err != nil {
		req.Signal = "15" // default SIGTERM
	}
	if req.Signal == "" {
		req.Signal = "15"
	}

	// Validate signal and pid are numeric to prevent injection
	for _, ch := range req.Signal {
		if ch < '0' || ch > '9' {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error":   true,
				"message": "Signal must be numeric",
			})
		}
	}
	for _, ch := range pid {
		if ch < '0' || ch > '9' {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error":   true,
				"message": "PID must be numeric",
			})
		}
	}

	cmd := fmt.Sprintf("kill -%s %s", req.Signal, pid)
	output, err := h.execSSH(serverID, cmd)
	if err != nil {
		return c.Status(fiber.StatusBadGateway).JSON(fiber.Map{
			"error":   true,
			"message": "Failed to kill process: " + err.Error(),
			"output":  output,
		})
	}

	return c.JSON(fiber.Map{
		"message": fmt.Sprintf("Signal %s sent to PID %s", req.Signal, pid),
		"output":  output,
	})
}

// ListServices returns systemd service units.
func (h *ProcessHandler) ListServices(c *fiber.Ctx) error {
	serverID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error":   true,
			"message": "Invalid server ID",
		})
	}

	output, err := h.execSSH(serverID, "systemctl list-units --type=service --state=running,failed,inactive --no-pager --plain | head -100")
	if err != nil {
		// Some systems may still return partial output even on error
		if output == "" {
			return c.Status(fiber.StatusBadGateway).JSON(fiber.Map{
				"error":   true,
				"message": "Failed to list services: " + err.Error(),
			})
		}
	}

	services := parseServices(output)
	return c.JSON(fiber.Map{"services": services})
}

// ServiceAction performs a systemctl action on a service.
func (h *ProcessHandler) ServiceAction(c *fiber.Ctx) error {
	serverID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error":   true,
			"message": "Invalid server ID",
		})
	}

	name := c.Params("name")
	if name == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error":   true,
			"message": "Service name is required",
		})
	}

	var req struct {
		Action string `json:"action"`
	}
	if err := c.BodyParser(&req); err != nil || req.Action == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error":   true,
			"message": "Action is required (restart, start, stop, enable, disable)",
		})
	}

	// Validate action
	validActions := map[string]bool{
		"restart": true, "start": true, "stop": true,
		"enable": true, "disable": true,
	}
	if !validActions[req.Action] {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error":   true,
			"message": "Invalid action. Must be: restart, start, stop, enable, disable",
		})
	}

	// Sanitize service name (only allow alphanumeric, dash, underscore, dot, @)
	for _, ch := range name {
		if !((ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') || (ch >= '0' && ch <= '9') || ch == '-' || ch == '_' || ch == '.' || ch == '@') {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error":   true,
				"message": "Invalid service name characters",
			})
		}
	}

	cmd := fmt.Sprintf("systemctl %s %s", req.Action, name)
	output, err := h.execSSH(serverID, cmd)
	if err != nil {
		return c.Status(fiber.StatusBadGateway).JSON(fiber.Map{
			"error":   true,
			"message": "Service action failed: " + err.Error(),
			"output":  output,
		})
	}

	return c.JSON(fiber.Map{
		"message": fmt.Sprintf("Service %s: %s", name, req.Action),
		"output":  output,
	})
}

// ListNetworkConnections returns active network connections.
func (h *ProcessHandler) ListNetworkConnections(c *fiber.Ctx) error {
	serverID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error":   true,
			"message": "Invalid server ID",
		})
	}

	output, err := h.execSSH(serverID, "ss -tunapl --no-header | head -100")
	if err != nil {
		if output == "" {
			return c.Status(fiber.StatusBadGateway).JSON(fiber.Map{
				"error":   true,
				"message": "Failed to list connections: " + err.Error(),
			})
		}
	}

	connections := parseNetworkConnections(output)
	return c.JSON(fiber.Map{"connections": connections})
}

// parseProcesses parses `ps aux` output into structured data.
// Fields: USER PID %CPU %MEM VSZ RSS TTY STAT START TIME COMMAND
// COMMAND is everything from field index 10 onward.
func parseProcesses(output string) []fiber.Map {
	lines := strings.Split(strings.TrimSpace(output), "\n")
	var processes []fiber.Map

	for i, line := range lines {
		if i == 0 {
			continue // skip header
		}
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		fields := strings.Fields(line)
		if len(fields) < 11 {
			continue
		}

		command := strings.Join(fields[10:], " ")

		processes = append(processes, fiber.Map{
			"user":    fields[0],
			"pid":     fields[1],
			"cpu":     fields[2],
			"mem":     fields[3],
			"vsz":     fields[4],
			"rss":     fields[5],
			"tty":     fields[6],
			"stat":    fields[7],
			"start":   fields[8],
			"time":    fields[9],
			"command": command,
		})
	}

	return processes
}

// parseServices parses `systemctl list-units` output.
// Fields: UNIT LOAD ACTIVE SUB DESCRIPTION (description may contain spaces)
func parseServices(output string) []fiber.Map {
	lines := strings.Split(strings.TrimSpace(output), "\n")
	var services []fiber.Map

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "UNIT") || strings.Contains(line, "loaded units listed") {
			continue
		}

		fields := strings.Fields(line)
		if len(fields) < 5 {
			continue
		}

		description := strings.Join(fields[4:], " ")

		services = append(services, fiber.Map{
			"name":        fields[0],
			"load":        fields[1],
			"active":      fields[2],
			"sub":         fields[3],
			"description": description,
		})
	}

	return services
}

// parseNetworkConnections parses `ss -tunapl` output.
func parseNetworkConnections(output string) []fiber.Map {
	lines := strings.Split(strings.TrimSpace(output), "\n")
	var connections []fiber.Map

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		fields := strings.Fields(line)
		if len(fields) < 5 {
			continue
		}

		conn := fiber.Map{
			"protocol":    fields[0],
			"state":       fields[1],
			"recv_q":      fields[2],
			"send_q":      fields[3],
			"local_addr":  fields[4],
			"remote_addr": "",
			"process":     "",
		}

		if len(fields) > 5 {
			conn["remote_addr"] = fields[5]
		}
		if len(fields) > 6 {
			conn["process"] = strings.Join(fields[6:], " ")
		}

		connections = append(connections, conn)
	}

	return connections
}
