package tools

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/ahmetk3436/bastion/internal/config"
	"github.com/ahmetk3436/bastion/internal/models"
	"github.com/google/uuid"
	"golang.org/x/crypto/ssh"
	"gorm.io/gorm"
)

// ToolCall represents a tool call from the AI
type ToolCall struct {
	ID       string           `json:"id"`
	Type     string           `json:"type"`
	Function ToolCallFunction `json:"function"`
}

// ToolCallFunction represents the function part of a tool call
type ToolCallFunction struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

// SSHPoolInterface defines the interface for SSH pool operations
type SSHPoolInterface interface {
	GetConnection(host string, port int, username, password, privateKey, authType string) (*ssh.Client, error)
}

// CredentialDecryptor defines the interface for decrypting credentials
type CredentialDecryptor interface {
	Decrypt(ciphertext string) (string, error)
}

// ToolRegistry manages available tools for AI function calling
type ToolRegistry struct {
	cfg        *config.Config
	db         *gorm.DB
	sshPool    SSHPoolInterface
	decryptor  CredentialDecryptor
	httpClient *http.Client
}

// NewToolRegistry creates a new tool registry
func NewToolRegistry(cfg *config.Config, db *gorm.DB, sshPool SSHPoolInterface, decryptor CredentialDecryptor) *ToolRegistry {
	return &ToolRegistry{
		cfg:       cfg,
		db:        db,
		sshPool:   sshPool,
		decryptor: decryptor,
		httpClient: &http.Client{
			Timeout: 60 * time.Second,
		},
	}
}

// GetToolDefinitions returns all available tools in OpenAI-compatible format
func (r *ToolRegistry) GetToolDefinitions() []map[string]interface{} {
	return []map[string]interface{}{
		r.executeCommandTool(),
		r.getServerListTool(),
		r.getMonitorStatusTool(),
		r.getLogsTool(),
		r.restartAppTool(),
		r.searchWebTool(),
	}
}

// executeCommandTool defines the execute_command tool
func (r *ToolRegistry) executeCommandTool() map[string]interface{} {
	return map[string]interface{}{
		"type": "function",
		"function": map[string]interface{}{
			"name":        "execute_command",
			"description": "Execute a shell command on a server via SSH. Use this to run diagnostic commands, check logs, restart services, or gather system information.",
			"parameters": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"command": map[string]interface{}{
						"type":        "string",
						"description": "The shell command to execute (e.g., 'docker ps', 'systemctl status nginx', 'tail -100 /var/log/app.log')",
					},
					"server_id": map[string]interface{}{
						"type":        "string",
						"description": "The UUID of the server to execute the command on. If omitted, uses the default server.",
					},
				},
				"required": []string{"command"},
			},
		},
	}
}

// getServerListTool defines the get_server_list tool
func (r *ToolRegistry) getServerListTool() map[string]interface{} {
	return map[string]interface{}{
		"type": "function",
		"function": map[string]interface{}{
			"name":        "get_server_list",
			"description": "Get a list of all configured servers with their status (online/offline), names, and connection details.",
			"parameters": map[string]interface{}{
				"type":       "object",
				"properties": map[string]interface{}{},
				"required":   []string{},
			},
		},
	}
}

// getMonitorStatusTool defines the get_monitor_status tool
func (r *ToolRegistry) getMonitorStatusTool() map[string]interface{} {
	return map[string]interface{}{
		"type": "function",
		"function": map[string]interface{}{
			"name":        "get_monitor_status",
			"description": "Get the latest monitoring status for a server including CPU, memory, disk usage, load average, and container counts.",
			"parameters": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"server_id": map[string]interface{}{
						"type":        "string",
						"description": "The UUID of the server. If omitted, uses the default server.",
					},
				},
				"required": []string{},
			},
		},
	}
}

// getLogsTool defines the get_logs tool
func (r *ToolRegistry) getLogsTool() map[string]interface{} {
	return map[string]interface{}{
		"type": "function",
		"function": map[string]interface{}{
			"name":        "get_logs",
			"description": "Fetch container logs from Coolify for a specific application.",
			"parameters": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"app_uuid": map[string]interface{}{
						"type":        "string",
						"description": "The UUID of the Coolify application to fetch logs for.",
					},
					"lines": map[string]interface{}{
						"type":        "integer",
						"description": "Number of log lines to retrieve (default: 100).",
					},
				},
				"required": []string{"app_uuid"},
			},
		},
	}
}

// restartAppTool defines the restart_app tool
func (r *ToolRegistry) restartAppTool() map[string]interface{} {
	return map[string]interface{}{
		"type": "function",
		"function": map[string]interface{}{
			"name":        "restart_app",
			"description": "Restart an application via Coolify API. Use this when an app is not responding or needs to be restarted after configuration changes.",
			"parameters": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"app_uuid": map[string]interface{}{
						"type":        "string",
						"description": "The UUID of the Coolify application to restart.",
					},
				},
				"required": []string{"app_uuid"},
			},
		},
	}
}

// searchWebTool defines the search_web tool
func (r *ToolRegistry) searchWebTool() map[string]interface{} {
	return map[string]interface{}{
		"type": "function",
		"function": map[string]interface{}{
			"name":        "search_web",
			"description": "Search the web for information about errors, documentation, or solutions. Use this when encountering unknown errors or needing to look up documentation.",
			"parameters": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"query": map[string]interface{}{
						"type":        "string",
						"description": "The search query (e.g., 'docker container exited with code 137', 'nginx 502 bad error troubleshooting')",
					},
				},
				"required": []string{"query"},
			},
		},
	}
}

// ExecuteTool executes a tool by name and returns the result
func (r *ToolRegistry) ExecuteTool(toolName string, arguments map[string]interface{}) (string, error) {
	switch toolName {
	case "execute_command":
		return r.executeCommand(arguments)
	case "get_server_list":
		return r.getServerList(arguments)
	case "get_monitor_status":
		return r.getMonitorStatus(arguments)
	case "get_logs":
		return r.getLogs(arguments)
	case "restart_app":
		return r.restartApp(arguments)
	case "search_web":
		return r.searchWeb(arguments)
	default:
		return "", fmt.Errorf("unknown tool: %s", toolName)
	}
}

// executeCommand implementation
func (r *ToolRegistry) executeCommand(args map[string]interface{}) (string, error) {
	command, _ := args["command"].(string)
	if command == "" {
		return "", fmt.Errorf("command is required")
	}

	// Get server_id or use default
	var server *models.Server
	serverIDStr, hasServerID := args["server_id"].(string)

	if hasServerID && serverIDStr != "" {
		serverID, err := uuid.Parse(serverIDStr)
		if err != nil {
			return "", fmt.Errorf("invalid server_id: %w", err)
		}
		if err := r.db.First(&server, "id = ?", serverID).Error; err != nil {
			return "", fmt.Errorf("server not found: %w", err)
		}
	} else {
		// Use default server
		if err := r.db.First(&server, "is_default = ?", true).Error; err != nil {
			// Try any server
			if err := r.db.First(&server).Error; err != nil {
				return "", fmt.Errorf("no server configured")
			}
		}
	}

	password, privateKey, err := r.decryptCredentials(server)
	if err != nil {
		return "", fmt.Errorf("failed to decrypt credentials: %w", err)
	}

	client, err := r.sshPool.GetConnection(server.Host, server.Port, server.Username, password, privateKey, server.AuthType)
	if err != nil {
		return "", fmt.Errorf("SSH connection failed: %w", err)
	}

	session, err := client.NewSession()
	if err != nil {
		return "", fmt.Errorf("failed to create SSH session: %w", err)
	}
	defer session.Close()

	var stdout, stderr bytes.Buffer
	session.Stdout = &stdout
	session.Stderr = &stderr

	if err := session.Run(command); err != nil {
		// Command failed but return output anyway
		output := stdout.String()
		errOutput := stderr.String()
		if errOutput != "" {
			if output != "" {
				output += "\n"
			}
			output += errOutput
		}
		return output + fmt.Sprintf("\n[Exit status: %v]", err), nil
	}

	output := stdout.String()
	if stderr.Len() > 0 {
		if output != "" {
			output += "\n"
		}
		output += stderr.String()
	}

	return output, nil
}

// decryptCredentials decrypts the password or private key for a server
func (r *ToolRegistry) decryptCredentials(server *models.Server) (password, privateKey string, err error) {
	if server.AuthType == "key" && server.EncryptedPrivateKey != "" {
		decrypted, err := r.decryptor.Decrypt(server.EncryptedPrivateKey)
		if err != nil {
			return "", "", err
		}
		privateKey = decrypted
	} else if server.EncryptedPassword != "" {
		decrypted, err := r.decryptor.Decrypt(server.EncryptedPassword)
		if err != nil {
			return "", "", err
		}
		password = decrypted
	}
	return password, privateKey, nil
}

// getServerList implementation
func (r *ToolRegistry) getServerList(args map[string]interface{}) (string, error) {
	var servers []models.Server
	if err := r.db.Order("name ASC").Find(&servers).Error; err != nil {
		return "", fmt.Errorf("failed to fetch servers: %w", err)
	}

	if len(servers) == 0 {
		return "No servers configured.", nil
	}

	var result string
	for i, s := range servers {
		result += fmt.Sprintf("%d. %s\n", i+1, s.Name)
		result += fmt.Sprintf("   Host: %s:%d\n", s.Host, s.Port)
		result += fmt.Sprintf("   Status: %s\n", s.Status)
		if s.IsDefault {
			result += "   [DEFAULT]\n"
		}
		result += fmt.Sprintf("   UUID: %s\n", s.ID.String())
		if i < len(servers)-1 {
			result += "\n"
		}
	}

	return result, nil
}

// getMonitorStatus implementation
func (r *ToolRegistry) getMonitorStatus(args map[string]interface{}) (string, error) {
	var server *models.Server
	serverIDStr, hasServerID := args["server_id"].(string)

	if hasServerID && serverIDStr != "" {
		serverID, err := uuid.Parse(serverIDStr)
		if err != nil {
			return "", fmt.Errorf("invalid server_id: %w", err)
		}
		if err := r.db.First(&server, "id = ?", serverID).Error; err != nil {
			return "", fmt.Errorf("server not found: %w", err)
		}
	} else {
		// Use default server
		if err := r.db.First(&server, "is_default = ?", true).Error; err != nil {
			if err := r.db.First(&server).Error; err != nil {
				return "", fmt.Errorf("no server configured")
			}
		}
	}

	var metrics models.ServerMetrics
	if err := r.db.Where("server_id = ?", server.ID).Order("collected_at DESC").First(&metrics).Error; err != nil {
		return fmt.Sprintf("No metrics available for server %s", server.Name), nil
	}

	result := fmt.Sprintf("Monitor Status for %s (collected %s)\n", server.Name, metrics.CollectedAt.Format("2006-01-02 15:04:05"))
	result += "─────────────────────────────────────\n"
	result += fmt.Sprintf("CPU:         %.1f%%\n", metrics.CPUPercent)
	result += fmt.Sprintf("Memory:      %.0f MB / %.0f MB (%.1f%%)\n",
		metrics.MemoryUsedMB, metrics.MemoryTotalMB,
		safePercent(metrics.MemoryUsedMB, metrics.MemoryTotalMB))
	result += fmt.Sprintf("Disk:        %.1f GB / %.1f GB (%.1f%%)\n",
		metrics.DiskUsedGB, metrics.DiskTotalGB,
		safePercent(metrics.DiskUsedGB, metrics.DiskTotalGB))
	result += fmt.Sprintf("Load Avg:    %.2f / %.2f / %.2f\n",
		metrics.LoadAvg1m, metrics.LoadAvg5m, metrics.LoadAvg15m)
	result += fmt.Sprintf("Containers:  %d running / %d total\n",
		metrics.ContainerRunning, metrics.ContainerCount)

	uptime := metrics.UptimeSeconds
	days := uptime / 86400
	hours := (uptime % 86400) / 3600
	mins := (uptime % 3600) / 60
	result += "Uptime:      "
	if days > 0 {
		result += fmt.Sprintf("%dd %dh %dm\n", days, hours, mins)
	} else if hours > 0 {
		result += fmt.Sprintf("%dh %dm\n", hours, mins)
	} else {
		result += fmt.Sprintf("%dm\n", mins)
	}

	return result, nil
}

// getLogs implementation
func (r *ToolRegistry) getLogs(args map[string]interface{}) (string, error) {
	appUUID, _ := args["app_uuid"].(string)
	if appUUID == "" {
		return "", fmt.Errorf("app_uuid is required")
	}

	lines := 100
	if linesArg, ok := args["lines"].(float64); ok {
		lines = int(linesArg)
	}

	url := fmt.Sprintf("%s/api/v1/applications/%s/logs?lines=%d", r.cfg.CoolifyAPIURL, appUUID, lines)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Authorization", r.cfg.CoolifyAPIToken)
	req.Header.Set("Accept", "application/json")

	resp, err := r.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to fetch logs: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("Coolify API returned status %d: %s", resp.StatusCode, string(body))
	}

	// Parse response
	var logsData map[string]interface{}
	if err := json.Unmarshal(body, &logsData); err != nil {
		// Return raw body if not JSON
		return string(body), nil
	}

	// Format logs nicely
	logsJSON, _ := json.MarshalIndent(logsData, "", "  ")
	return string(logsJSON), nil
}

// restartApp implementation
func (r *ToolRegistry) restartApp(args map[string]interface{}) (string, error) {
	appUUID, _ := args["app_uuid"].(string)
	if appUUID == "" {
		return "", fmt.Errorf("app_uuid is required")
	}

	url := fmt.Sprintf("%s/api/v1/applications/%s/restart", r.cfg.CoolifyAPIURL, appUUID)
	req, err := http.NewRequest("POST", url, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Authorization", r.cfg.CoolifyAPIToken)
	req.Header.Set("Accept", "application/json")

	resp, err := r.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to restart app: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("Coolify API returned status %d: %s", resp.StatusCode, string(body))
	}

	return fmt.Sprintf("App %s restart initiated successfully", appUUID), nil
}

// searchWeb implementation - placeholder for now
func (r *ToolRegistry) searchWeb(args map[string]interface{}) (string, error) {
	query, _ := args["query"].(string)
	if query == "" {
		return "", fmt.Errorf("query is required")
	}

	slog.Info("Web search requested", "query", query)

	// For now, return a message indicating web search capability
	// TODO: Integrate with Tavily/Serper API
	return fmt.Sprintf("Web search for '%s' was requested. To enable actual web search, configure Tavily or Serper API keys.", query), nil
}

// Helper functions
func safePercent(used, total float64) float64 {
	if total == 0 {
		return 0
	}
	return (used / total) * 100
}

// ParseToolCallID extracts the tool call ID from various response formats
func ParseToolCallID(toolCall interface{}) string {
	switch t := toolCall.(type) {
	case map[string]interface{}:
		if id, ok := t["id"].(string); ok {
			return id
		}
	case ToolCall:
		return t.ID
	}
	return ""
}
