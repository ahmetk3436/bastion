package services

import (
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/ahmetk3436/bastion/internal/models"
	"gorm.io/gorm"
)

type MonitorChecker struct {
	db   *gorm.DB
	stop chan struct{}
}

func NewMonitorChecker(db *gorm.DB) *MonitorChecker {
	return &MonitorChecker{
		db:   db,
		stop: make(chan struct{}),
	}
}

func (mc *MonitorChecker) Start() {
	go mc.loop()
	slog.Info("Monitor checker started")
}

func (mc *MonitorChecker) Stop() {
	mc.stop <- struct{}{}
	slog.Info("Monitor checker stopped")
}

func (mc *MonitorChecker) loop() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	// Run an initial check on startup
	mc.checkAll()

	for {
		select {
		case <-ticker.C:
			mc.checkAll()
		case <-mc.stop:
			return
		}
	}
}

func (mc *MonitorChecker) checkAll() {
	var monitors []models.Monitor
	mc.db.Where("enabled = ?", true).Find(&monitors)

	for _, m := range monitors {
		if m.LastCheckedAt != nil && time.Since(*m.LastCheckedAt) < time.Duration(m.IntervalSeconds)*time.Second {
			continue
		}
		go mc.checkOne(m)
	}
}

func (mc *MonitorChecker) checkOne(m models.Monitor) {
	start := time.Now()
	client := &http.Client{Timeout: time.Duration(m.TimeoutMs) * time.Millisecond}

	ping := models.MonitorPing{
		MonitorID: m.ID,
		CheckedAt: time.Now(),
	}

	req, err := http.NewRequest(m.Method, m.URL, nil)
	if err != nil {
		ping.Status = "down"
		ping.Error = fmt.Sprintf("invalid request: %s", err.Error())
		ping.ResponseMs = int(time.Since(start).Milliseconds())
		mc.savePing(m, ping)
		return
	}

	resp, err := client.Do(req)
	responseMs := int(time.Since(start).Milliseconds())
	ping.ResponseMs = responseMs

	if err != nil {
		ping.Status = "down"
		ping.Error = err.Error()
	} else {
		defer resp.Body.Close()
		ping.StatusCode = resp.StatusCode
		if resp.StatusCode == m.ExpectedStatus {
			ping.Status = "up"
		} else {
			ping.Status = "down"
			ping.Error = fmt.Sprintf("expected %d, got %d", m.ExpectedStatus, resp.StatusCode)
		}
	}

	mc.savePing(m, ping)
}

func (mc *MonitorChecker) savePing(m models.Monitor, ping models.MonitorPing) {
	if err := mc.db.Create(&ping).Error; err != nil {
		slog.Error("Failed to save monitor ping", "monitor", m.Name, "error", err)
		return
	}

	now := time.Now()
	updates := map[string]interface{}{
		"last_checked_at":  now,
		"last_status":      ping.Status,
		"last_response_ms": ping.ResponseMs,
	}

	if ping.Status == "down" {
		updates["consecutive_fails"] = gorm.Expr("consecutive_fails + 1")
	} else {
		updates["consecutive_fails"] = 0
	}

	// Calculate uptime percent from recent pings (last 100)
	var totalPings, upPings int64
	mc.db.Model(&models.MonitorPing{}).Where("monitor_id = ?", m.ID).Count(&totalPings)
	mc.db.Model(&models.MonitorPing{}).Where("monitor_id = ? AND status = ?", m.ID, "up").Count(&upPings)

	if totalPings > 0 {
		updates["uptime_percent"] = float64(upPings) / float64(totalPings) * 100
	}

	mc.db.Model(&models.Monitor{}).Where("id = ?", m.ID).Updates(updates)
}
