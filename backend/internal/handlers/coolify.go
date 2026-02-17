package handlers

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/ahmetk3436/bastion/internal/config"
	"github.com/gofiber/fiber/v2"
)

type CoolifyHandler struct {
	cfg    *config.Config
	client *http.Client
}

func NewCoolifyHandler(cfg *config.Config) *CoolifyHandler {
	return &CoolifyHandler{
		cfg: cfg,
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

func (h *CoolifyHandler) proxyGet(path string) ([]byte, int, error) {
	url := fmt.Sprintf("%s/api/v1/%s", h.cfg.CoolifyAPIURL, path)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("Authorization", h.cfg.CoolifyAPIToken)
	req.Header.Set("Accept", "application/json")

	resp, err := h.client.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	return body, resp.StatusCode, err
}

func (h *CoolifyHandler) proxyPost(path string, jsonBody []byte) ([]byte, int, error) {
	url := fmt.Sprintf("%s/api/v1/%s", h.cfg.CoolifyAPIURL, path)
	var bodyReader io.Reader
	if jsonBody != nil {
		bodyReader = strings.NewReader(string(jsonBody))
	}
	req, err := http.NewRequest("POST", url, bodyReader)
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("Authorization", h.cfg.CoolifyAPIToken)
	req.Header.Set("Accept", "application/json")
	if jsonBody != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := h.client.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	return body, resp.StatusCode, err
}

func (h *CoolifyHandler) ListApps(c *fiber.Ctx) error {
	body, status, err := h.proxyGet("applications")
	if err != nil {
		slog.Error("Coolify list apps failed", "error", err)
		return c.Status(fiber.StatusBadGateway).JSON(fiber.Map{
			"error":   true,
			"message": "Failed to connect to Coolify",
		})
	}

	var result interface{}
	json.Unmarshal(body, &result)
	return c.Status(status).JSON(result)
}

func (h *CoolifyHandler) GetApp(c *fiber.Ctx) error {
	uuid := c.Params("uuid")
	body, status, err := h.proxyGet(fmt.Sprintf("applications/%s", uuid))
	if err != nil {
		return c.Status(fiber.StatusBadGateway).JSON(fiber.Map{
			"error":   true,
			"message": "Failed to connect to Coolify",
		})
	}

	var result interface{}
	json.Unmarshal(body, &result)
	return c.Status(status).JSON(result)
}

func (h *CoolifyHandler) RestartApp(c *fiber.Ctx) error {
	uuid := c.Params("uuid")
	body, status, err := h.proxyPost(fmt.Sprintf("applications/%s/restart", uuid), nil)
	if err != nil {
		return c.Status(fiber.StatusBadGateway).JSON(fiber.Map{
			"error":   true,
			"message": "Failed to restart app via Coolify",
		})
	}

	var result interface{}
	json.Unmarshal(body, &result)
	return c.Status(status).JSON(result)
}

func (h *CoolifyHandler) DeployApp(c *fiber.Ctx) error {
	uuid := c.Params("uuid")
	reqBody, _ := json.Marshal(map[string]interface{}{
		"uuid":  uuid,
		"force": true,
	})
	body, status, err := h.proxyPost("deploy", reqBody)
	if err != nil {
		return c.Status(fiber.StatusBadGateway).JSON(fiber.Map{
			"error":   true,
			"message": "Failed to deploy via Coolify",
		})
	}

	var result interface{}
	json.Unmarshal(body, &result)
	return c.Status(status).JSON(result)
}

func (h *CoolifyHandler) GetAppLogs(c *fiber.Ctx) error {
	uuid := c.Params("uuid")
	body, status, err := h.proxyGet(fmt.Sprintf("applications/%s/logs", uuid))
	if err != nil {
		return c.Status(fiber.StatusBadGateway).JSON(fiber.Map{
			"error":   true,
			"message": "Failed to get logs from Coolify",
		})
	}

	var result interface{}
	json.Unmarshal(body, &result)
	return c.Status(status).JSON(result)
}

func (h *CoolifyHandler) GetAppEnvs(c *fiber.Ctx) error {
	uuid := c.Params("uuid")
	body, status, err := h.proxyGet(fmt.Sprintf("applications/%s/envs", uuid))
	if err != nil {
		return c.Status(fiber.StatusBadGateway).JSON(fiber.Map{
			"error":   true,
			"message": "Failed to get envs from Coolify",
		})
	}

	var result interface{}
	json.Unmarshal(body, &result)
	return c.Status(status).JSON(result)
}

func (h *CoolifyHandler) UpdateAppEnvs(c *fiber.Ctx) error {
	uuid := c.Params("uuid")
	reqBody := c.Body()
	url := fmt.Sprintf("%s/api/v1/applications/%s/envs", h.cfg.CoolifyAPIURL, uuid)

	req, _ := http.NewRequest("PATCH", url, strings.NewReader(string(reqBody)))
	req.Header.Set("Authorization", h.cfg.CoolifyAPIToken)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := h.client.Do(req)
	if err != nil {
		return c.Status(fiber.StatusBadGateway).JSON(fiber.Map{
			"error":   true,
			"message": "Failed to update envs via Coolify",
		})
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var result interface{}
	json.Unmarshal(body, &result)
	return c.Status(resp.StatusCode).JSON(result)
}

func (h *CoolifyHandler) ListDatabases(c *fiber.Ctx) error {
	body, status, err := h.proxyGet("databases")
	if err != nil {
		return c.Status(fiber.StatusBadGateway).JSON(fiber.Map{
			"error":   true,
			"message": "Failed to list databases from Coolify",
		})
	}

	var result interface{}
	json.Unmarshal(body, &result)
	return c.Status(status).JSON(result)
}

func (h *CoolifyHandler) ListServices(c *fiber.Ctx) error {
	body, status, err := h.proxyGet("services")
	if err != nil {
		return c.Status(fiber.StatusBadGateway).JSON(fiber.Map{
			"error":   true,
			"message": "Failed to list services from Coolify",
		})
	}

	var result interface{}
	json.Unmarshal(body, &result)
	return c.Status(status).JSON(result)
}

func (h *CoolifyHandler) ListDeployments(c *fiber.Ctx) error {
	body, status, err := h.proxyGet("deployments")
	if err != nil {
		return c.Status(fiber.StatusBadGateway).JSON(fiber.Map{
			"error":   true,
			"message": "Failed to list deployments from Coolify",
		})
	}

	var result interface{}
	json.Unmarshal(body, &result)
	return c.Status(status).JSON(result)
}
