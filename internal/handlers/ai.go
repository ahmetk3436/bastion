package handlers

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/ahmetk3436/bastion/internal/config"
	"github.com/ahmetk3436/bastion/internal/models"
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/valyala/fasthttp"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

// ─── Coolify Apps Cache ─────────────────────────────────────────────────────

type coolifyCache struct {
	mu        sync.RWMutex
	apps      string
	fetchedAt time.Time
}

var appCache = &coolifyCache{}

const coolifyAppCacheTTL = 5 * time.Minute

// ─── AIHandler ──────────────────────────────────────────────────────────────

type AIHandler struct {
	cfg           *config.Config
	db            *gorm.DB
	client        *http.Client
	streamClient  *http.Client // no timeout for streaming
	serverHandler *ServerHandler
}

func NewAIHandler(cfg *config.Config, db *gorm.DB, serverHandler *ServerHandler) *AIHandler {
	return &AIHandler{
		cfg: cfg,
		db:  db,
		client: &http.Client{
			Timeout: 120 * time.Second,
		},
		streamClient: &http.Client{
			Timeout: 0, // no timeout for SSE streaming
		},
		serverHandler: serverHandler,
	}
}

// ─── Types ──────────────────────────────────────────────────────────────────

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type AIActionRequest struct {
	Action   string `json:"action"`   // "execute_command", "restart_app", "get_logs", "get_metrics"
	ServerID string `json:"server_id"`
	Command  string `json:"command"`  // for execute_command
	AppUUID  string `json:"app_uuid"` // for restart_app, get_logs
}

// ─── Chat (non-streaming) ───────────────────────────────────────────────────

func (h *AIHandler) Chat(c *fiber.Ctx) error {
	var req struct {
		Message        string `json:"message"`
		ConversationID string `json:"conversation_id"`
		ServerID       string `json:"server_id"`
	}
	if err := c.BodyParser(&req); err != nil || req.Message == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error":   true,
			"message": "Message is required",
		})
	}

	// Load or create conversation
	var conv models.AIConversation
	var messages []chatMessage

	if req.ConversationID != "" {
		convID, _ := uuid.Parse(req.ConversationID)
		if err := h.db.First(&conv, "id = ?", convID).Error; err == nil {
			json.Unmarshal([]byte(conv.Messages), &messages)
		}
	}

	var serverID *uuid.UUID
	if req.ServerID != "" {
		sid, _ := uuid.Parse(req.ServerID)
		serverID = &sid
	}

	if conv.ID == uuid.Nil {
		conv = models.AIConversation{
			Title:    truncate(req.Message, 100),
			Messages: datatypes.JSON("[]"),
		}
		if serverID != nil {
			conv.ServerID = serverID
		}
		h.db.Create(&conv)
	}

	messages = append(messages, chatMessage{Role: "user", Content: req.Message})

	// Build context-aware system prompt
	systemPrompt := h.buildSystemPrompt(serverID)

	// Determine if thinking mode should be enabled
	useThinking := isComplexQuery(req.Message)

	glmMessages := make([]map[string]string, 0, len(messages)+1)
	glmMessages = append(glmMessages, map[string]string{"role": "system", "content": systemPrompt})
	for _, m := range messages {
		glmMessages = append(glmMessages, map[string]string{"role": m.Role, "content": m.Content})
	}

	glmReq := map[string]interface{}{
		"model":    h.cfg.GLMModel,
		"messages": glmMessages,
		"stream":   false,
	}
	if useThinking {
		glmReq["thinking"] = map[string]string{"type": "enabled"}
	}

	body, _ := json.Marshal(glmReq)
	httpReq, _ := http.NewRequest("POST", h.cfg.GLMAPIURL, bytes.NewReader(body))
	httpReq.Header.Set("Authorization", "Bearer "+h.cfg.GLMAPIKey)
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := h.client.Do(httpReq)
	if err != nil {
		slog.Error("GLM-5 API call failed", "error", err)
		return c.Status(fiber.StatusBadGateway).JSON(fiber.Map{
			"error":   true,
			"message": "AI service unavailable",
		})
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)

	var glmResp struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	json.Unmarshal(respBody, &glmResp)

	aiResponse := "I couldn't generate a response. Please try again."
	if len(glmResp.Choices) > 0 {
		aiResponse = glmResp.Choices[0].Message.Content
	}

	messages = append(messages, chatMessage{Role: "assistant", Content: aiResponse})
	msgJSON, _ := json.Marshal(messages)
	h.db.Model(&conv).Update("messages", datatypes.JSON(msgJSON))

	return c.JSON(fiber.Map{
		"response":        aiResponse,
		"conversation_id": conv.ID,
	})
}

// ─── ChatStream (SSE Streaming) ─────────────────────────────────────────────

func (h *AIHandler) ChatStream(c *fiber.Ctx) error {
	var req struct {
		Message        string `json:"message"`
		ConversationID string `json:"conversation_id"`
		ServerID       string `json:"server_id"`
	}
	if err := c.BodyParser(&req); err != nil || req.Message == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error":   true,
			"message": "Message is required",
		})
	}

	// Load or create conversation
	var conv models.AIConversation
	var messages []chatMessage

	if req.ConversationID != "" {
		convID, _ := uuid.Parse(req.ConversationID)
		if err := h.db.First(&conv, "id = ?", convID).Error; err == nil {
			json.Unmarshal([]byte(conv.Messages), &messages)
		}
	}

	var serverID *uuid.UUID
	if req.ServerID != "" {
		sid, _ := uuid.Parse(req.ServerID)
		serverID = &sid
	}

	if conv.ID == uuid.Nil {
		conv = models.AIConversation{
			Title:    truncate(req.Message, 100),
			Messages: datatypes.JSON("[]"),
		}
		if serverID != nil {
			conv.ServerID = serverID
		}
		h.db.Create(&conv)
	}

	messages = append(messages, chatMessage{Role: "user", Content: req.Message})

	// Build context-aware system prompt
	systemPrompt := h.buildSystemPrompt(serverID)

	// Determine if thinking mode should be enabled
	useThinking := isComplexQuery(req.Message)

	glmMessages := make([]map[string]string, 0, len(messages)+1)
	glmMessages = append(glmMessages, map[string]string{"role": "system", "content": systemPrompt})
	for _, m := range messages {
		glmMessages = append(glmMessages, map[string]string{"role": m.Role, "content": m.Content})
	}

	glmReq := map[string]interface{}{
		"model":    h.cfg.GLMModel,
		"messages": glmMessages,
		"stream":   true,
	}
	if useThinking {
		glmReq["thinking"] = map[string]string{"type": "enabled"}
	}

	glmBody, _ := json.Marshal(glmReq)
	httpReq, err := http.NewRequest("POST", h.cfg.GLMAPIURL, bytes.NewReader(glmBody))
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error":   true,
			"message": "Failed to create request",
		})
	}
	httpReq.Header.Set("Authorization", "Bearer "+h.cfg.GLMAPIKey)
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "text/event-stream")

	resp, err := h.streamClient.Do(httpReq)
	if err != nil {
		slog.Error("GLM-5 streaming call failed", "error", err)
		return c.Status(fiber.StatusBadGateway).JSON(fiber.Map{
			"error":   true,
			"message": "AI service unavailable",
		})
	}

	// Set SSE headers
	c.Set("Content-Type", "text/event-stream")
	c.Set("Cache-Control", "no-cache")
	c.Set("Connection", "keep-alive")
	c.Set("X-Accel-Buffering", "no")

	// Capture conv data for the closure
	convID := conv.ID
	dbRef := h.db
	allMessages := messages

	c.Context().SetBodyStreamWriter(func(w *bufio.Writer) {
		defer resp.Body.Close()

		scanner := bufio.NewScanner(resp.Body)
		// Increase scanner buffer for large chunks
		scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

		var fullResponse strings.Builder

		for scanner.Scan() {
			line := scanner.Text()

			// Skip empty lines and comments
			if line == "" || strings.HasPrefix(line, ":") {
				continue
			}

			// Only process data lines
			if !strings.HasPrefix(line, "data: ") {
				continue
			}

			data := strings.TrimPrefix(line, "data: ")

			// Check for end of stream
			if data == "[DONE]" {
				break
			}

			// Parse the SSE chunk from GLM
			var chunk struct {
				Choices []struct {
					Delta struct {
						Content string `json:"content"`
					} `json:"delta"`
					FinishReason *string `json:"finish_reason"`
				} `json:"choices"`
			}

			if err := json.Unmarshal([]byte(data), &chunk); err != nil {
				continue
			}

			if len(chunk.Choices) > 0 {
				token := chunk.Choices[0].Delta.Content
				if token != "" {
					fullResponse.WriteString(token)

					// Forward token to client
					event := map[string]interface{}{
						"token": token,
						"done":  false,
					}
					eventJSON, _ := json.Marshal(event)
					fmt.Fprintf(w, "data: %s\n\n", eventJSON)
					w.Flush()
				}

				// Check if this is the final chunk
				if chunk.Choices[0].FinishReason != nil {
					break
				}
			}
		}

		// Send final event
		finalEvent := map[string]interface{}{
			"token":           "",
			"done":            true,
			"conversation_id": convID.String(),
		}
		finalJSON, _ := json.Marshal(finalEvent)
		fmt.Fprintf(w, "data: %s\n\n", finalJSON)
		w.Flush()

		// Save the full assembled response to the conversation
		assembled := fullResponse.String()
		if assembled == "" {
			assembled = "I couldn't generate a response. Please try again."
		}

		allMessages = append(allMessages, chatMessage{Role: "assistant", Content: assembled})
		msgJSON, _ := json.Marshal(allMessages)
		dbRef.Model(&models.AIConversation{}).Where("id = ?", convID).Update("messages", datatypes.JSON(msgJSON))
	})

	return nil
}

// ─── ExecuteAIAction ────────────────────────────────────────────────────────

func (h *AIHandler) ExecuteAIAction(c *fiber.Ctx) error {
	var req AIActionRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error":   true,
			"message": "Invalid request body",
		})
	}

	switch req.Action {
	case "execute_command":
		return h.executeCommand(c, req)
	case "restart_app":
		return h.restartApp(c, req)
	case "get_logs":
		return h.getLogs(c, req)
	case "get_metrics":
		return h.getMetrics(c, req)
	default:
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error":   true,
			"message": "Unknown action: " + req.Action + ". Valid actions: execute_command, restart_app, get_logs, get_metrics",
		})
	}
}

func (h *AIHandler) executeCommand(c *fiber.Ctx, req AIActionRequest) error {
	if req.ServerID == "" || req.Command == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error":   true,
			"message": "server_id and command are required for execute_command",
		})
	}

	serverID, err := uuid.Parse(req.ServerID)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error":   true,
			"message": "Invalid server_id",
		})
	}

	var server models.Server
	if err := h.db.First(&server, "id = ?", serverID).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error":   true,
			"message": "Server not found",
		})
	}

	password, privateKey, err := h.serverHandler.GetDecryptedCredentials(&server)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error":   true,
			"message": "Failed to decrypt server credentials",
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
		if exitErr, ok := err.(interface{ ExitStatus() int }); ok {
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

	// Save to command history
	history := models.CommandHistory{
		ServerID:   serverID,
		Command:    req.Command,
		Output:     output,
		ExitCode:   exitCode,
		ExecutedAt: start,
		DurationMs: int(duration.Milliseconds()),
	}
	h.db.Create(&history)

	return c.JSON(fiber.Map{
		"action":      "execute_command",
		"command":     req.Command,
		"output":      output,
		"exit_code":   exitCode,
		"duration_ms": duration.Milliseconds(),
		"server":      server.Name,
	})
}

func (h *AIHandler) restartApp(c *fiber.Ctx, req AIActionRequest) error {
	if req.AppUUID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error":   true,
			"message": "app_uuid is required for restart_app",
		})
	}

	url := fmt.Sprintf("%s/api/v1/applications/%s/restart", h.cfg.CoolifyAPIURL, req.AppUUID)
	httpReq, _ := http.NewRequest("POST", url, nil)
	httpReq.Header.Set("Authorization", h.cfg.CoolifyAPIToken)
	httpReq.Header.Set("Accept", "application/json")

	resp, err := h.client.Do(httpReq)
	if err != nil {
		return c.Status(fiber.StatusBadGateway).JSON(fiber.Map{
			"error":   true,
			"message": "Failed to restart app via Coolify: " + err.Error(),
		})
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var result interface{}
	json.Unmarshal(body, &result)

	return c.JSON(fiber.Map{
		"action":   "restart_app",
		"app_uuid": req.AppUUID,
		"status":   resp.StatusCode,
		"result":   result,
	})
}

func (h *AIHandler) getLogs(c *fiber.Ctx, req AIActionRequest) error {
	if req.AppUUID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error":   true,
			"message": "app_uuid is required for get_logs",
		})
	}

	url := fmt.Sprintf("%s/api/v1/applications/%s/logs", h.cfg.CoolifyAPIURL, req.AppUUID)
	httpReq, _ := http.NewRequest("GET", url, nil)
	httpReq.Header.Set("Authorization", h.cfg.CoolifyAPIToken)
	httpReq.Header.Set("Accept", "application/json")

	resp, err := h.client.Do(httpReq)
	if err != nil {
		return c.Status(fiber.StatusBadGateway).JSON(fiber.Map{
			"error":   true,
			"message": "Failed to get logs from Coolify: " + err.Error(),
		})
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var result interface{}
	json.Unmarshal(body, &result)

	return c.JSON(fiber.Map{
		"action":   "get_logs",
		"app_uuid": req.AppUUID,
		"result":   result,
	})
}

func (h *AIHandler) getMetrics(c *fiber.Ctx, req AIActionRequest) error {
	if req.ServerID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error":   true,
			"message": "server_id is required for get_metrics",
		})
	}

	serverID, err := uuid.Parse(req.ServerID)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error":   true,
			"message": "Invalid server_id",
		})
	}

	var metrics models.ServerMetrics
	if err := h.db.Where("server_id = ?", serverID).Order("collected_at DESC").First(&metrics).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error":   true,
			"message": "No metrics available for this server",
		})
	}

	return c.JSON(fiber.Map{
		"action":  "get_metrics",
		"metrics": metrics,
	})
}

// ─── GetConversation ────────────────────────────────────────────────────────

func (h *AIHandler) GetConversation(c *fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error":   true,
			"message": "Invalid conversation ID",
		})
	}

	var conv models.AIConversation
	if err := h.db.First(&conv, "id = ?", id).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error":   true,
			"message": "Conversation not found",
		})
	}

	// Parse messages from JSON
	var messages []chatMessage
	json.Unmarshal([]byte(conv.Messages), &messages)

	return c.JSON(fiber.Map{
		"id":         conv.ID,
		"title":      conv.Title,
		"server_id":  conv.ServerID,
		"messages":   messages,
		"created_at": conv.CreatedAt,
		"updated_at": conv.UpdatedAt,
	})
}

// ─── AnalyzeLogs ────────────────────────────────────────────────────────────

func (h *AIHandler) AnalyzeLogs(c *fiber.Ctx) error {
	var req struct {
		Logs     string `json:"logs"`
		Context  string `json:"context"`
		ServerID string `json:"server_id"`
	}
	if err := c.BodyParser(&req); err != nil || req.Logs == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error":   true,
			"message": "Logs are required",
		})
	}

	prompt := fmt.Sprintf(`Analyze these server/application logs and identify:
1. Errors and their root causes
2. Warning patterns
3. Performance issues
4. Recommended actions

Context: %s

Logs:
%s`, req.Context, req.Logs)

	glmReq := map[string]interface{}{
		"model": h.cfg.GLMModel,
		"messages": []map[string]string{
			{"role": "system", "content": "You are a DevOps log analysis expert. Analyze logs concisely, identify issues, and suggest fixes with specific commands."},
			{"role": "user", "content": prompt},
		},
		"stream":   false,
		"thinking": map[string]string{"type": "enabled"},
	}

	body, _ := json.Marshal(glmReq)
	httpReq, _ := http.NewRequest("POST", h.cfg.GLMAPIURL, bytes.NewReader(body))
	httpReq.Header.Set("Authorization", "Bearer "+h.cfg.GLMAPIKey)
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := h.client.Do(httpReq)
	if err != nil {
		return c.Status(fiber.StatusBadGateway).JSON(fiber.Map{
			"error":   true,
			"message": "AI service unavailable",
		})
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	var glmResp struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	json.Unmarshal(respBody, &glmResp)

	analysis := "Unable to analyze logs."
	if len(glmResp.Choices) > 0 {
		analysis = glmResp.Choices[0].Message.Content
	}

	return c.JSON(fiber.Map{"analysis": analysis})
}

// ─── SuggestFix ─────────────────────────────────────────────────────────────

func (h *AIHandler) SuggestFix(c *fiber.Ctx) error {
	var req struct {
		Error   string `json:"error"`
		Context string `json:"context"`
	}
	if err := c.BodyParser(&req); err != nil || req.Error == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error":   true,
			"message": "Error description is required",
		})
	}

	prompt := fmt.Sprintf(`An error occurred in a server/application. Suggest a fix with specific commands.

Error: %s
Context: %s

Provide:
1. Root cause analysis
2. Immediate fix (commands to run)
3. Prevention strategy`, req.Error, req.Context)

	glmReq := map[string]interface{}{
		"model": h.cfg.GLMModel,
		"messages": []map[string]string{
			{"role": "system", "content": "You are a DevOps troubleshooting expert. Provide concise, actionable fixes with specific commands."},
			{"role": "user", "content": prompt},
		},
		"stream":   false,
		"thinking": map[string]string{"type": "enabled"},
	}

	body, _ := json.Marshal(glmReq)
	httpReq, _ := http.NewRequest("POST", h.cfg.GLMAPIURL, bytes.NewReader(body))
	httpReq.Header.Set("Authorization", "Bearer "+h.cfg.GLMAPIKey)
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := h.client.Do(httpReq)
	if err != nil {
		return c.Status(fiber.StatusBadGateway).JSON(fiber.Map{
			"error":   true,
			"message": "AI service unavailable",
		})
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	var glmResp struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	json.Unmarshal(respBody, &glmResp)

	suggestion := "Unable to suggest a fix."
	if len(glmResp.Choices) > 0 {
		suggestion = glmResp.Choices[0].Message.Content
	}

	return c.JSON(fiber.Map{"suggestion": suggestion})
}

// ─── ListConversations ──────────────────────────────────────────────────────

func (h *AIHandler) ListConversations(c *fiber.Ctx) error {
	page, _ := strconv.Atoi(c.Query("page", "1"))
	perPage, _ := strconv.Atoi(c.Query("per_page", "20"))
	if page < 1 {
		page = 1
	}
	if perPage < 1 || perPage > 50 {
		perPage = 20
	}

	var convs []models.AIConversation
	var total int64
	h.db.Model(&models.AIConversation{}).Count(&total)
	h.db.Order("updated_at DESC").Offset((page - 1) * perPage).Limit(perPage).Find(&convs)

	// Strip messages to save bandwidth
	type convSummary struct {
		ID        uuid.UUID  `json:"id"`
		Title     string     `json:"title"`
		ServerID  *uuid.UUID `json:"server_id"`
		CreatedAt time.Time  `json:"created_at"`
		UpdatedAt time.Time  `json:"updated_at"`
	}
	summaries := make([]convSummary, len(convs))
	for i, conv := range convs {
		summaries[i] = convSummary{
			ID:        conv.ID,
			Title:     conv.Title,
			ServerID:  conv.ServerID,
			CreatedAt: conv.CreatedAt,
			UpdatedAt: conv.UpdatedAt,
		}
	}

	return c.JSON(fiber.Map{
		"conversations": summaries,
		"total":         total,
		"page":          page,
		"per_page":      perPage,
	})
}

// ─── DeleteConversation ─────────────────────────────────────────────────────

func (h *AIHandler) DeleteConversation(c *fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error":   true,
			"message": "Invalid conversation ID",
		})
	}

	h.db.Delete(&models.AIConversation{}, "id = ?", id)
	return c.JSON(fiber.Map{"message": "Conversation deleted"})
}

// ─── Context-Aware System Prompt Builder ────────────────────────────────────

func (h *AIHandler) buildSystemPrompt(serverID *uuid.UUID) string {
	var sb strings.Builder

	sb.WriteString(`You are Bastion AI, a powerful DevOps assistant for Ahmet's infrastructure.

## Your Capabilities
- Execute commands on servers via SSH
- Check server metrics (CPU, RAM, disk, network, load average, containers)
- View and analyze container logs from Coolify
- Restart containers and apps via Coolify API
- Check deployment status and history
- Analyze errors and suggest fixes with root cause analysis
- Monitor SRE events and alerts

## Guidelines
- Be concise and technical
- Format commands as ` + "```bash" + ` code blocks
- When suggesting a command to run, include [Execute] at the end of the code block
- Always consider security implications before suggesting destructive commands
- Provide root cause analysis for errors
- Use metrics data to support your analysis
- If a server is offline, mention it and suggest diagnostics
`)

	// Add server-specific context if serverID provided
	if serverID != nil {
		var server models.Server
		if err := h.db.First(&server, "id = ?", *serverID).Error; err == nil {
			sb.WriteString(fmt.Sprintf("\n## Current Server Context\n"))
			sb.WriteString(fmt.Sprintf("- **Name**: %s\n", server.Name))
			sb.WriteString(fmt.Sprintf("- **Host**: %s:%d\n", server.Host, server.Port))
			sb.WriteString(fmt.Sprintf("- **Status**: %s\n", server.Status))
			if server.LastConnectedAt != nil {
				sb.WriteString(fmt.Sprintf("- **Last Connected**: %s\n", server.LastConnectedAt.Format(time.RFC3339)))
			}

			// Get latest metrics for this server
			var metrics models.ServerMetrics
			if err := h.db.Where("server_id = ?", *serverID).Order("collected_at DESC").First(&metrics).Error; err == nil {
				sb.WriteString(fmt.Sprintf("\n### Latest Metrics (collected %s)\n", metrics.CollectedAt.Format("15:04:05 MST")))
				sb.WriteString(fmt.Sprintf("- CPU: %.1f%%\n", metrics.CPUPercent))
				sb.WriteString(fmt.Sprintf("- Memory: %.0f MB / %.0f MB (%.1f%%)\n",
					metrics.MemoryUsedMB, metrics.MemoryTotalMB,
					safePercent(metrics.MemoryUsedMB, metrics.MemoryTotalMB)))
				sb.WriteString(fmt.Sprintf("- Disk: %.1f GB / %.1f GB (%.1f%%)\n",
					metrics.DiskUsedGB, metrics.DiskTotalGB,
					safePercent(metrics.DiskUsedGB, metrics.DiskTotalGB)))
				sb.WriteString(fmt.Sprintf("- Load Average: %.2f / %.2f / %.2f\n",
					metrics.LoadAvg1m, metrics.LoadAvg5m, metrics.LoadAvg15m))
				sb.WriteString(fmt.Sprintf("- Containers: %d running / %d total\n",
					metrics.ContainerRunning, metrics.ContainerCount))
				sb.WriteString(fmt.Sprintf("- Uptime: %s\n", formatUptime(metrics.UptimeSeconds)))
			}
		}
	}

	// Add all servers list
	var servers []models.Server
	if err := h.db.Order("name ASC").Find(&servers).Error; err == nil && len(servers) > 0 {
		sb.WriteString("\n## All Servers\n")
		for _, s := range servers {
			statusIcon := "?"
			switch s.Status {
			case "online":
				statusIcon = "UP"
			case "offline":
				statusIcon = "DOWN"
			}
			sb.WriteString(fmt.Sprintf("- [%s] %s (%s:%d) ID: %s\n", statusIcon, s.Name, s.Host, s.Port, s.ID))
		}
	}

	// Add Coolify running apps (cached)
	if apps := h.getCoolifyAppsContext(); apps != "" {
		sb.WriteString("\n## Running Coolify Apps\n")
		sb.WriteString(apps)
		sb.WriteString("\n")
	}

	// Add recent SRE events from ops backend
	if events := h.getRecentSREEvents(); events != "" {
		sb.WriteString("\n## Recent SRE Events (last 10)\n")
		sb.WriteString(events)
		sb.WriteString("\n")
	}

	return sb.String()
}

func (h *AIHandler) getCoolifyAppsContext() string {
	appCache.mu.RLock()
	if time.Since(appCache.fetchedAt) < coolifyAppCacheTTL && appCache.apps != "" {
		cached := appCache.apps
		appCache.mu.RUnlock()
		return cached
	}
	appCache.mu.RUnlock()

	// Fetch from Coolify
	url := fmt.Sprintf("%s/api/v1/applications", h.cfg.CoolifyAPIURL)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return ""
	}
	req.Header.Set("Authorization", h.cfg.CoolifyAPIToken)
	req.Header.Set("Accept", "application/json")

	fetchClient := &http.Client{Timeout: 10 * time.Second}
	resp, err := fetchClient.Do(req)
	if err != nil {
		slog.Debug("Failed to fetch Coolify apps for AI context", "error", err)
		return ""
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	// Parse as array of objects
	var apps []map[string]interface{}
	if err := json.Unmarshal(body, &apps); err != nil {
		return ""
	}

	var sb strings.Builder
	for _, app := range apps {
		name, _ := app["name"].(string)
		appUUID, _ := app["uuid"].(string)
		status, _ := app["status"].(string)
		if name == "" {
			continue
		}
		sb.WriteString(fmt.Sprintf("- %s (uuid: %s, status: %s)\n", name, appUUID, status))
	}

	result := sb.String()

	// Update cache
	appCache.mu.Lock()
	appCache.apps = result
	appCache.fetchedAt = time.Now()
	appCache.mu.Unlock()

	return result
}

func (h *AIHandler) getRecentSREEvents() string {
	if h.cfg.OpsBackendURL == "" || h.cfg.OpsAdminToken == "" {
		return ""
	}

	url := fmt.Sprintf("%s/api/ops/sre/events?per_page=10", h.cfg.OpsBackendURL)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return ""
	}
	req.Header.Set("X-Admin-Token", h.cfg.OpsAdminToken)
	req.Header.Set("Accept", "application/json")

	fetchClient := &http.Client{Timeout: 10 * time.Second}
	resp, err := fetchClient.Do(req)
	if err != nil {
		slog.Debug("Failed to fetch SRE events for AI context", "error", err)
		return ""
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	var data struct {
		Events []struct {
			ContainerName string `json:"container_name"`
			Pattern       string `json:"pattern"`
			Severity      string `json:"severity"`
			CreatedAt     string `json:"created_at"`
			Message       string `json:"message"`
		} `json:"events"`
	}
	if err := json.Unmarshal(body, &data); err != nil {
		return ""
	}

	if len(data.Events) == 0 {
		return "No recent SRE events.\n"
	}

	var sb strings.Builder
	for _, evt := range data.Events {
		msg := truncate(evt.Message, 120)
		sb.WriteString(fmt.Sprintf("- [%s] %s: %s — %s (%s)\n",
			evt.Severity, evt.ContainerName, evt.Pattern, msg, evt.CreatedAt))
	}

	return sb.String()
}

// ─── Helpers ────────────────────────────────────────────────────────────────

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

func safePercent(used, total float64) float64 {
	if total == 0 {
		return 0
	}
	return (used / total) * 100
}

func formatUptime(seconds int64) string {
	days := seconds / 86400
	hours := (seconds % 86400) / 3600
	mins := (seconds % 3600) / 60
	if days > 0 {
		return fmt.Sprintf("%dd %dh %dm", days, hours, mins)
	}
	if hours > 0 {
		return fmt.Sprintf("%dh %dm", hours, mins)
	}
	return fmt.Sprintf("%dm", mins)
}

// isComplexQuery returns true if the user's message warrants GLM-5 thinking mode.
func isComplexQuery(message string) bool {
	lower := strings.ToLower(message)
	complexKeywords := []string{
		"analyze", "debug", "fix", "diagnose", "investigate",
		"root cause", "why is", "performance", "optimize",
		"crash", "error", "failing", "down", "not working",
		"architecture", "design", "plan", "strategy", "migrate",
		"compare", "explain", "troubleshoot",
	}
	for _, kw := range complexKeywords {
		if strings.Contains(lower, kw) {
			return true
		}
	}
	return false
}

// Ensure fasthttp import is used by the stream writer signature
var _ = (*fasthttp.RequestCtx)(nil)
