package main

import (
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/ahmetk3436/bastion/internal/config"
	"github.com/ahmetk3436/bastion/internal/crypto"
	"github.com/ahmetk3436/bastion/internal/database"
	"github.com/ahmetk3436/bastion/internal/handlers"
	"github.com/ahmetk3436/bastion/internal/routes"
	"github.com/ahmetk3436/bastion/internal/services"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/recover"
)

func main() {
	// JSON structured logging
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	slog.SetDefault(logger)

	slog.Info("Starting Bastion", "version", handlers.Version)

	// ─── Config ──────────────────────────────────────────────────────────
	cfg := config.Load()

	// ─── Database ────────────────────────────────────────────────────────
	if err := database.Connect(cfg); err != nil {
		slog.Error("Database connection failed", "error", err)
		os.Exit(1)
	}

	if err := database.Migrate(); err != nil {
		slog.Error("Database migration failed", "error", err)
		os.Exit(1)
	}

	db := database.DB

	// ─── Encryption ─────────────────────────────────────────────────────
	var encryptor *crypto.Encryptor
	if cfg.SSHEncryptionKey != "" {
		var err error
		encryptor, err = crypto.NewEncryptor(cfg.SSHEncryptionKey)
		if err != nil {
			slog.Error("Failed to create encryptor", "error", err)
			os.Exit(1)
		}
		slog.Info("SSH credential encryption initialized")
	} else {
		slog.Warn("SSH_ENCRYPTION_KEY not set, credentials will not be encrypted")
		// Create a dummy encryptor with a default key for development
		encryptor, _ = crypto.NewEncryptor("0000000000000000000000000000000000000000000000000000000000000000")
	}

	// ─── SSH Pool ───────────────────────────────────────────────────────
	sshPool := services.NewSSHPool()

	// ─── Metrics Collector ──────────────────────────────────────────────
	metricsCollector := services.NewMetricsCollector(db, sshPool, encryptor, cfg.MetricsCollectInterval)
	metricsCollector.Start()

	// ─── Monitor Checker ────────────────────────────────────────────────
	monitorChecker := services.NewMonitorChecker(db)
	monitorChecker.Start()

	// ─── Handlers ───────────────────────────────────────────────────────
	authHandler := handlers.NewAuthHandler(cfg)
	serverHandler := handlers.NewServerHandler(db, encryptor, sshPool)
	terminalHandler := handlers.NewTerminalHandler(serverHandler)
	commandHandler := handlers.NewCommandHandler(serverHandler)
	cronHandler := handlers.NewCronHandler(db, serverHandler)
	coolifyHandler := handlers.NewCoolifyHandler(cfg)
	opsHandler := handlers.NewOpsHandler(cfg)
	aiHandler := handlers.NewAIHandler(cfg, db, serverHandler)
	systemHandler := handlers.NewSystemHandler(db, cfg)
	processHandler := handlers.NewProcessHandler(serverHandler)
	dockerHandler := handlers.NewDockerHandler(serverHandler)
	monitorHandler := handlers.NewMonitorHandler(db)
	alertHandler := handlers.NewAlertHandler(db)
	databaseHandler := handlers.NewDatabaseHandler(db)
	fileHandler := handlers.NewFileHandler(serverHandler)
	auditHandler := handlers.NewAuditHandler(db)

	// ─── Fiber App ──────────────────────────────────────────────────────
	app := fiber.New(fiber.Config{
		AppName:      "bastion v" + handlers.Version,
		ServerHeader: "bastion",
		BodyLimit:    10 * 1024 * 1024, // 10MB for log uploads
		ErrorHandler: func(c *fiber.Ctx, err error) error {
			code := fiber.StatusInternalServerError
			message := "Internal server error"
			if e, ok := err.(*fiber.Error); ok {
				code = e.Code
				message = e.Message
			}
			return c.Status(code).JSON(fiber.Map{
				"error":   true,
				"message": message,
			})
		},
	})

	app.Use(cors.New(cors.Config{
		AllowOrigins: "*",
		AllowHeaders: "Origin, Content-Type, Accept, Authorization",
		AllowMethods: "GET, POST, PUT, DELETE, PATCH, OPTIONS",
	}))

	app.Use(recover.New(recover.Config{
		EnableStackTrace: false,
	}))

	// Security headers
	app.Use(func(c *fiber.Ctx) error {
		c.Set("X-Content-Type-Options", "nosniff")
		c.Set("X-Frame-Options", "DENY")
		c.Set("X-XSS-Protection", "1; mode=block")
		return c.Next()
	})

	// Request logger
	app.Use(func(c *fiber.Ctx) error {
		start := time.Now()
		err := c.Next()
		if c.Path() == "/api/health" {
			return err
		}
		slog.Info("request",
			"method", c.Method(),
			"path", c.Path(),
			"status", c.Response().StatusCode(),
			"duration_ms", time.Since(start).Milliseconds(),
			"ip", c.IP(),
		)
		return err
	})

	// ─── Routes ─────────────────────────────────────────────────────────
	routes.Setup(app, cfg, authHandler, serverHandler, terminalHandler, commandHandler,
		cronHandler, coolifyHandler, opsHandler, aiHandler, systemHandler,
		processHandler, dockerHandler, monitorHandler, alertHandler, databaseHandler,
		fileHandler, auditHandler)

	// ─── Graceful Shutdown ──────────────────────────────────────────────
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-quit
		slog.Info("Shutting down Bastion...")

		monitorChecker.Stop()
		metricsCollector.Stop()
		sshPool.CloseAll()

		if err := app.Shutdown(); err != nil {
			slog.Error("Fiber shutdown error", "error", err)
		}

		if sqlDB, err := database.DB.DB(); err == nil {
			sqlDB.Close()
		}
	}()

	// ─── Start ──────────────────────────────────────────────────────────
	listenAddr := ":" + cfg.Port
	slog.Info("Bastion listening", "addr", listenAddr)

	if err := app.Listen(listenAddr); err != nil {
		slog.Error("Server error", "error", err)
		os.Exit(1)
	}
}
