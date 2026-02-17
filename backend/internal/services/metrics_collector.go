package services

import (
	"log/slog"
	"strconv"
	"strings"
	"time"

	"github.com/ahmetk3436/bastion/internal/crypto"
	"github.com/ahmetk3436/bastion/internal/models"
	"golang.org/x/crypto/ssh"
	"gorm.io/gorm"
)

type MetricsCollector struct {
	db        *gorm.DB
	sshPool   *SSHPool
	encryptor *crypto.Encryptor
	interval  time.Duration
	stop      chan struct{}
}

func NewMetricsCollector(db *gorm.DB, pool *SSHPool, encryptor *crypto.Encryptor, intervalSecs int) *MetricsCollector {
	return &MetricsCollector{
		db:        db,
		sshPool:   pool,
		encryptor: encryptor,
		interval:  time.Duration(intervalSecs) * time.Second,
		stop:      make(chan struct{}),
	}
}

func (mc *MetricsCollector) Start() {
	go mc.loop()
	slog.Info("Metrics collector started", "interval", mc.interval)
}

func (mc *MetricsCollector) Stop() {
	close(mc.stop)
	slog.Info("Metrics collector stopped")
}

func (mc *MetricsCollector) loop() {
	// Initial collection
	mc.collectAll()

	ticker := time.NewTicker(mc.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			mc.collectAll()
		case <-mc.stop:
			return
		}
	}
}

func (mc *MetricsCollector) collectAll() {
	var servers []models.Server
	mc.db.Find(&servers)

	for _, server := range servers {
		go mc.collectServer(server)
	}
}

func (mc *MetricsCollector) CollectNow() {
	mc.collectAll()
}

func (mc *MetricsCollector) collectServer(server models.Server) {
	password, privateKey := "", ""
	if server.EncryptedPassword != "" {
		p, err := mc.encryptor.Decrypt(server.EncryptedPassword)
		if err == nil {
			password = p
		}
	}
	if server.EncryptedPrivateKey != "" {
		k, err := mc.encryptor.Decrypt(server.EncryptedPrivateKey)
		if err == nil {
			privateKey = k
		}
	}

	client, err := mc.sshPool.GetConnection(server.Host, server.Port, server.Username, password, privateKey, server.AuthType)
	if err != nil {
		mc.db.Model(&server).Update("status", "offline")
		slog.Debug("Metrics collection failed", "server", server.Name, "error", err)
		return
	}

	mc.db.Model(&server).Update("status", "online")

	metrics := models.ServerMetrics{
		ServerID:    server.ID,
		CollectedAt: time.Now(),
	}

	// CPU
	if out := runCommand(client, `top -bn1 | head -3 | grep 'Cpu' | awk '{print $2}'`); out != "" {
		metrics.CPUPercent, _ = strconv.ParseFloat(strings.TrimSpace(out), 64)
	}

	// Memory
	if out := runCommand(client, `free -m | awk 'NR==2{print $2" "$3}'`); out != "" {
		parts := strings.Fields(strings.TrimSpace(out))
		if len(parts) >= 2 {
			metrics.MemoryTotalMB, _ = strconv.ParseFloat(parts[0], 64)
			metrics.MemoryUsedMB, _ = strconv.ParseFloat(parts[1], 64)
		}
	}

	// Disk
	if out := runCommand(client, `df -BG / | awk 'NR==2{gsub("G",""); print $2" "$3}'`); out != "" {
		parts := strings.Fields(strings.TrimSpace(out))
		if len(parts) >= 2 {
			metrics.DiskTotalGB, _ = strconv.ParseFloat(parts[0], 64)
			metrics.DiskUsedGB, _ = strconv.ParseFloat(parts[1], 64)
		}
	}

	// Load average
	if out := runCommand(client, `cat /proc/loadavg | awk '{print $1" "$2" "$3}'`); out != "" {
		parts := strings.Fields(strings.TrimSpace(out))
		if len(parts) >= 3 {
			metrics.LoadAvg1m, _ = strconv.ParseFloat(parts[0], 64)
			metrics.LoadAvg5m, _ = strconv.ParseFloat(parts[1], 64)
			metrics.LoadAvg15m, _ = strconv.ParseFloat(parts[2], 64)
		}
	}

	// Uptime
	if out := runCommand(client, `cat /proc/uptime | awk '{print int($1)}'`); out != "" {
		metrics.UptimeSeconds, _ = strconv.ParseInt(strings.TrimSpace(out), 10, 64)
	}

	// Docker containers
	if out := runCommand(client, `docker ps -a --format '{{.Status}}' 2>/dev/null | wc -l`); out != "" {
		count, _ := strconv.Atoi(strings.TrimSpace(out))
		metrics.ContainerCount = count
	}
	if out := runCommand(client, `docker ps --format '{{.Status}}' 2>/dev/null | wc -l`); out != "" {
		count, _ := strconv.Atoi(strings.TrimSpace(out))
		metrics.ContainerRunning = count
	}

	// Network (bytes since boot)
	if out := runCommand(client, `cat /proc/net/dev | awk 'NR>2{rx+=$2; tx+=$10} END{print rx" "tx}'`); out != "" {
		parts := strings.Fields(strings.TrimSpace(out))
		if len(parts) >= 2 {
			metrics.NetworkRxBytes, _ = strconv.ParseInt(parts[0], 10, 64)
			metrics.NetworkTxBytes, _ = strconv.ParseInt(parts[1], 10, 64)
		}
	}

	mc.db.Create(&metrics)
	slog.Debug("Metrics collected", "server", server.Name, "cpu", metrics.CPUPercent, "mem_used", metrics.MemoryUsedMB)
}

func runCommand(client *ssh.Client, cmd string) string {
	session, err := client.NewSession()
	if err != nil {
		return ""
	}
	defer session.Close()

	out, err := session.CombinedOutput(cmd)
	if err != nil {
		return ""
	}
	return string(out)
}
