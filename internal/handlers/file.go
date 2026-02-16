package handlers

import (
	"fmt"
	"strings"

	"github.com/ahmetk3436/bastion/internal/models"
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
)

type FileHandler struct {
	serverHandler *ServerHandler
}

func NewFileHandler(serverHandler *ServerHandler) *FileHandler {
	return &FileHandler{serverHandler: serverHandler}
}

func (h *FileHandler) execSSH(serverID uuid.UUID, command string) (string, error) {
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

// sanitizePath validates the path does not contain shell injection characters.
func sanitizePath(path string) bool {
	// Disallow shell metacharacters
	dangerous := []string{";", "&", "|", "$", "`", "'", "\"", "(", ")", "{", "}", "<", ">", "\n", "\r"}
	for _, ch := range dangerous {
		if strings.Contains(path, ch) {
			return false
		}
	}
	return path != "" && len(path) <= 4096
}

// ListFiles returns directory listing.
func (h *FileHandler) ListFiles(c *fiber.Ctx) error {
	serverID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error":   true,
			"message": "Invalid server ID",
		})
	}

	path := c.Query("path", "/")
	if !sanitizePath(path) {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error":   true,
			"message": "Invalid path",
		})
	}

	cmd := fmt.Sprintf("ls -la %s", path)
	output, err := h.execSSH(serverID, cmd)
	if err != nil {
		return c.Status(fiber.StatusBadGateway).JSON(fiber.Map{
			"error":   true,
			"message": "Failed to list files: " + err.Error(),
		})
	}

	files := parseFileList(output)
	return c.JSON(fiber.Map{
		"path":  path,
		"files": files,
	})
}

// ReadFile returns the content of a file (limited to 1MB).
func (h *FileHandler) ReadFile(c *fiber.Ctx) error {
	serverID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error":   true,
			"message": "Invalid server ID",
		})
	}

	path := c.Query("path", "")
	if path == "" || !sanitizePath(path) {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error":   true,
			"message": "Valid path is required",
		})
	}

	// Read file with 1MB limit using head
	cmd := fmt.Sprintf("head -c 1048576 %s", path)
	output, err := h.execSSH(serverID, cmd)
	if err != nil {
		return c.Status(fiber.StatusBadGateway).JSON(fiber.Map{
			"error":   true,
			"message": "Failed to read file: " + err.Error(),
		})
	}

	// Get file size
	sizeCmd := fmt.Sprintf("stat -c %%s %s 2>/dev/null || stat -f %%z %s 2>/dev/null", path, path)
	sizeOutput, _ := h.execSSH(serverID, sizeCmd)
	sizeOutput = strings.TrimSpace(sizeOutput)

	return c.JSON(fiber.Map{
		"path":      path,
		"content":   output,
		"size":      sizeOutput,
		"truncated": len(output) >= 1048576,
	})
}

// WriteFile writes content to a file on the server.
func (h *FileHandler) WriteFile(c *fiber.Ctx) error {
	serverID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error":   true,
			"message": "Invalid server ID",
		})
	}

	var req struct {
		Path    string `json:"path"`
		Content string `json:"content"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error":   true,
			"message": "Invalid request body",
		})
	}

	if req.Path == "" || !sanitizePath(req.Path) {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error":   true,
			"message": "Valid path is required",
		})
	}

	// Use a heredoc to write file content to avoid shell escaping issues
	// We use base64 to safely transfer content
	var server models.Server
	if err := h.serverHandler.GetDB().First(&server, "id = ?", serverID).Error; err != nil {
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

	client, err := h.serverHandler.GetSSHPool().GetConnection(server.Host, server.Port, server.Username, password, privateKey, server.AuthType)
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
			"message": "SSH session failed",
		})
	}
	defer session.Close()

	// Write content via stdin pipe
	stdin, err := session.StdinPipe()
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error":   true,
			"message": "Failed to get stdin pipe",
		})
	}

	cmd := fmt.Sprintf("cat > %s", req.Path)
	if err := session.Start(cmd); err != nil {
		return c.Status(fiber.StatusBadGateway).JSON(fiber.Map{
			"error":   true,
			"message": "Failed to start write command: " + err.Error(),
		})
	}

	if _, err := stdin.Write([]byte(req.Content)); err != nil {
		return c.Status(fiber.StatusBadGateway).JSON(fiber.Map{
			"error":   true,
			"message": "Failed to write content: " + err.Error(),
		})
	}
	stdin.Close()

	if err := session.Wait(); err != nil {
		return c.Status(fiber.StatusBadGateway).JSON(fiber.Map{
			"error":   true,
			"message": "Write command failed: " + err.Error(),
		})
	}

	return c.JSON(fiber.Map{
		"message": "File written successfully",
		"path":    req.Path,
		"size":    len(req.Content),
	})
}

// DiskUsage returns disk usage information.
func (h *FileHandler) DiskUsage(c *fiber.Ctx) error {
	serverID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error":   true,
			"message": "Invalid server ID",
		})
	}

	// Get df -h output
	dfOutput, err := h.execSSH(serverID, "df -h")
	if err != nil {
		return c.Status(fiber.StatusBadGateway).JSON(fiber.Map{
			"error":   true,
			"message": "Failed to get disk usage: " + err.Error(),
		})
	}

	// Get top-level directory sizes
	duOutput, _ := h.execSSH(serverID, "du -sh /* 2>/dev/null | sort -rh | head -10")

	filesystems := parseDfOutput(dfOutput)
	topDirs := parseDuOutput(duOutput)

	return c.JSON(fiber.Map{
		"filesystems": filesystems,
		"top_dirs":    topDirs,
	})
}

// parseFileList parses `ls -la` output.
func parseFileList(output string) []fiber.Map {
	lines := strings.Split(strings.TrimSpace(output), "\n")
	var files []fiber.Map

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "total") {
			continue
		}

		fields := strings.Fields(line)
		if len(fields) < 9 {
			continue
		}

		name := strings.Join(fields[8:], " ")
		isDir := strings.HasPrefix(fields[0], "d")

		files = append(files, fiber.Map{
			"permissions": fields[0],
			"links":       fields[1],
			"owner":       fields[2],
			"group":       fields[3],
			"size":        fields[4],
			"modified":    fmt.Sprintf("%s %s %s", fields[5], fields[6], fields[7]),
			"name":        name,
			"is_dir":      isDir,
		})
	}

	return files
}

// parseDfOutput parses `df -h` output.
func parseDfOutput(output string) []fiber.Map {
	lines := strings.Split(strings.TrimSpace(output), "\n")
	var filesystems []fiber.Map

	for i, line := range lines {
		if i == 0 {
			continue // skip header
		}
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		fields := strings.Fields(line)
		if len(fields) < 6 {
			continue
		}

		filesystems = append(filesystems, fiber.Map{
			"filesystem": fields[0],
			"size":       fields[1],
			"used":       fields[2],
			"available":  fields[3],
			"use_pct":    fields[4],
			"mounted_on": fields[5],
		})
	}

	return filesystems
}

// parseDuOutput parses `du -sh` output.
func parseDuOutput(output string) []fiber.Map {
	lines := strings.Split(strings.TrimSpace(output), "\n")
	var dirs []fiber.Map

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}

		dirs = append(dirs, fiber.Map{
			"size": fields[0],
			"path": fields[1],
		})
	}

	return dirs
}
