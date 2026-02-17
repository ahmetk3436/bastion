package handlers

import (
	"encoding/json"
	"log/slog"
	"time"

	"github.com/ahmetk3436/bastion/internal/models"
	"github.com/gofiber/contrib/websocket"
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"golang.org/x/crypto/ssh"
)

type TerminalHandler struct {
	serverHandler *ServerHandler
}

func NewTerminalHandler(serverHandler *ServerHandler) *TerminalHandler {
	return &TerminalHandler{serverHandler: serverHandler}
}

// UpgradeCheck is middleware that checks if the request is a websocket upgrade
func (h *TerminalHandler) UpgradeCheck() fiber.Handler {
	return func(c *fiber.Ctx) error {
		if websocket.IsWebSocketUpgrade(c) {
			return c.Next()
		}
		return fiber.ErrUpgradeRequired
	}
}

// HandleTerminal handles WebSocket terminal sessions
func (h *TerminalHandler) HandleTerminal() fiber.Handler {
	return websocket.New(func(c *websocket.Conn) {
		serverID, err := uuid.Parse(c.Params("id"))
		if err != nil {
			c.WriteMessage(websocket.TextMessage, []byte("Error: Invalid server ID"))
			return
		}

		db := h.serverHandler.GetDB()

		var server models.Server
		if err := db.First(&server, "id = ?", serverID).Error; err != nil {
			c.WriteMessage(websocket.TextMessage, []byte("Error: Server not found"))
			return
		}

		password, privateKey, err := h.serverHandler.GetDecryptedCredentials(&server)
		if err != nil {
			c.WriteMessage(websocket.TextMessage, []byte("Error: Failed to decrypt credentials"))
			return
		}

		pool := h.serverHandler.GetSSHPool()
		client, err := pool.GetConnection(server.Host, server.Port, server.Username, password, privateKey, server.AuthType)
		if err != nil {
			c.WriteMessage(websocket.TextMessage, []byte("Error: SSH connection failed: "+err.Error()))
			return
		}

		session, err := client.NewSession()
		if err != nil {
			c.WriteMessage(websocket.TextMessage, []byte("Error: Failed to create SSH session"))
			return
		}
		defer session.Close()

		// Record session
		sshSession := models.SSHSession{
			ServerID:  serverID,
			StartedAt: time.Now(),
		}
		db.Create(&sshSession)

		// Request PTY
		modes := ssh.TerminalModes{
			ssh.ECHO:          1,
			ssh.TTY_OP_ISPEED: 14400,
			ssh.TTY_OP_OSPEED: 14400,
		}

		cols := 80
		rows := 24

		if err := session.RequestPty("xterm-256color", rows, cols, modes); err != nil {
			c.WriteMessage(websocket.TextMessage, []byte("Error: Failed to request PTY"))
			return
		}

		// Get stdin/stdout pipes
		stdin, err := session.StdinPipe()
		if err != nil {
			c.WriteMessage(websocket.TextMessage, []byte("Error: Failed to get stdin pipe"))
			return
		}

		stdout, err := session.StdoutPipe()
		if err != nil {
			c.WriteMessage(websocket.TextMessage, []byte("Error: Failed to get stdout pipe"))
			return
		}

		stderr, err := session.StderrPipe()
		if err != nil {
			c.WriteMessage(websocket.TextMessage, []byte("Error: Failed to get stderr pipe"))
			return
		}

		if err := session.Shell(); err != nil {
			c.WriteMessage(websocket.TextMessage, []byte("Error: Failed to start shell"))
			return
		}

		slog.Info("Terminal session started", "server", server.Name, "host", server.Host)

		var bytesTransferred int64
		var commandsExecuted int

		done := make(chan struct{})

		// stdout → WebSocket
		go func() {
			buf := make([]byte, 4096)
			for {
				n, err := stdout.Read(buf)
				if err != nil {
					close(done)
					return
				}
				if n > 0 {
					bytesTransferred += int64(n)
					c.WriteMessage(websocket.TextMessage, buf[:n])
				}
			}
		}()

		// stderr → WebSocket
		go func() {
			buf := make([]byte, 4096)
			for {
				n, err := stderr.Read(buf)
				if err != nil {
					return
				}
				if n > 0 {
					bytesTransferred += int64(n)
					c.WriteMessage(websocket.TextMessage, buf[:n])
				}
			}
		}()

		// WebSocket → stdin
		go func() {
			for {
				msgType, msg, err := c.ReadMessage()
				if err != nil {
					session.Close()
					return
				}

				switch msgType {
				case websocket.TextMessage:
					// Check for control messages
					var ctrl struct {
						Type string `json:"type"`
						Cols int    `json:"cols"`
						Rows int    `json:"rows"`
					}
					if json.Unmarshal(msg, &ctrl) == nil && ctrl.Type == "resize" {
						session.WindowChange(ctrl.Rows, ctrl.Cols)
						continue
					}
					// Regular text input
					stdin.Write(msg)
					if len(msg) > 0 && msg[len(msg)-1] == '\r' {
						commandsExecuted++
					}
				case websocket.BinaryMessage:
					stdin.Write(msg)
					bytesTransferred += int64(len(msg))
				}
			}
		}()

		// Wait for session to end
		select {
		case <-done:
		}

		// Update session record
		now := time.Now()
		duration := int(now.Sub(sshSession.StartedAt).Seconds())
		db.Model(&sshSession).Updates(map[string]interface{}{
			"ended_at":          now,
			"duration_seconds":  duration,
			"commands_executed": commandsExecuted,
			"bytes_transferred": bytesTransferred,
		})

		// Update server last connected
		db.Model(&server).Update("last_connected_at", now)

		slog.Info("Terminal session ended", "server", server.Name, "duration", duration)
	})
}
