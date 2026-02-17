package handlers

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/ahmetk3436/bastion/internal/config"
	"github.com/gofiber/fiber/v2"
)

type OpsHandler struct {
	cfg    *config.Config
	client *http.Client
}

func NewOpsHandler(cfg *config.Config) *OpsHandler {
	return &OpsHandler{
		cfg: cfg,
		client: &http.Client{
			Timeout: 15 * time.Second,
		},
	}
}

func (h *OpsHandler) opsGet(path string) ([]byte, int, error) {
	url := fmt.Sprintf("%s%s", h.cfg.OpsBackendURL, path)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("X-Admin-Token", h.cfg.OpsAdminToken)
	req.Header.Set("Accept", "application/json")

	resp, err := h.client.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	return body, resp.StatusCode, err
}

func (h *OpsHandler) Overview(c *fiber.Ctx) error {
	// Fetch SRE stats, ticket stats, review stats in parallel
	type result struct {
		key    string
		data   interface{}
		status int
	}

	ch := make(chan result, 3)

	go func() {
		body, status, _ := h.opsGet("/api/ops/sre/stats")
		var data interface{}
		json.Unmarshal(body, &data)
		ch <- result{"sre", data, status}
	}()

	go func() {
		body, status, _ := h.opsGet("/api/ops/tickets?per_page=5")
		var data interface{}
		json.Unmarshal(body, &data)
		ch <- result{"tickets", data, status}
	}()

	go func() {
		body, status, _ := h.opsGet("/api/ops/reviews/stats")
		var data interface{}
		json.Unmarshal(body, &data)
		ch <- result{"reviews", data, status}
	}()

	overview := make(map[string]interface{})
	for i := 0; i < 3; i++ {
		r := <-ch
		overview[r.key] = r.data
	}

	return c.JSON(overview)
}

func (h *OpsHandler) SREEvents(c *fiber.Ctx) error {
	query := c.Request().URI().QueryString()
	path := "/api/ops/sre/events"
	if len(query) > 0 {
		path += "?" + string(query)
	}

	body, status, err := h.opsGet(path)
	if err != nil {
		return c.Status(fiber.StatusBadGateway).JSON(fiber.Map{
			"error":   true,
			"message": "Failed to connect to ops backend",
		})
	}

	var result interface{}
	json.Unmarshal(body, &result)
	return c.Status(status).JSON(result)
}

func (h *OpsHandler) Tickets(c *fiber.Ctx) error {
	query := c.Request().URI().QueryString()
	path := "/api/ops/tickets"
	if len(query) > 0 {
		path += "?" + string(query)
	}

	body, status, err := h.opsGet(path)
	if err != nil {
		return c.Status(fiber.StatusBadGateway).JSON(fiber.Map{
			"error":   true,
			"message": "Failed to connect to ops backend",
		})
	}

	var result interface{}
	json.Unmarshal(body, &result)
	return c.Status(status).JSON(result)
}

func (h *OpsHandler) Reviews(c *fiber.Ctx) error {
	query := c.Request().URI().QueryString()
	path := "/api/ops/reviews"
	if len(query) > 0 {
		path += "?" + string(query)
	}

	body, status, err := h.opsGet(path)
	if err != nil {
		return c.Status(fiber.StatusBadGateway).JSON(fiber.Map{
			"error":   true,
			"message": "Failed to connect to ops backend",
		})
	}

	var result interface{}
	json.Unmarshal(body, &result)
	return c.Status(status).JSON(result)
}
