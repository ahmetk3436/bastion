package routes

import (
	"github.com/ahmetk3436/bastion/internal/config"
	"github.com/ahmetk3436/bastion/internal/handlers"
	"github.com/ahmetk3436/bastion/internal/middleware"
	"github.com/gofiber/fiber/v2"
)

func Setup(
	app *fiber.App,
	cfg *config.Config,
	authHandler *handlers.AuthHandler,
	serverHandler *handlers.ServerHandler,
	terminalHandler *handlers.TerminalHandler,
	commandHandler *handlers.CommandHandler,
	cronHandler *handlers.CronHandler,
	coolifyHandler *handlers.CoolifyHandler,
	opsHandler *handlers.OpsHandler,
	aiHandler *handlers.AIHandler,
	systemHandler *handlers.SystemHandler,
) {
	// ─── Public ──────────────────────────────────────────────────────────
	app.Get("/api/health", systemHandler.Health)

	// ─── Auth ────────────────────────────────────────────────────────────
	app.Post("/api/auth/login", authHandler.Login)
	app.Post("/api/auth/refresh", authHandler.Refresh)

	// ─── Protected routes ────────────────────────────────────────────────
	api := app.Group("/api", middleware.JWTProtected(cfg.JWTSecret))

	// Auth (protected)
	api.Get("/auth/me", authHandler.Me)
	api.Put("/auth/password", authHandler.ChangePassword)

	// Dashboard
	api.Get("/dashboard/overview", systemHandler.DashboardOverview)

	// System
	api.Get("/system/info", systemHandler.Info)

	// Servers
	api.Get("/servers", serverHandler.ListServers)
	api.Post("/servers", serverHandler.CreateServer)
	api.Get("/servers/:id", serverHandler.GetServer)
	api.Put("/servers/:id", serverHandler.UpdateServer)
	api.Delete("/servers/:id", serverHandler.DeleteServer)
	api.Post("/servers/:id/test", serverHandler.TestConnection)
	api.Get("/servers/:id/metrics", serverHandler.GetMetrics)
	api.Get("/servers/:id/metrics/live", serverHandler.GetLiveMetrics)

	// Terminal (WebSocket)
	api.Use("/servers/:id/terminal", terminalHandler.UpgradeCheck())
	api.Get("/servers/:id/terminal", terminalHandler.HandleTerminal())

	// Commands
	api.Post("/servers/:id/exec", commandHandler.ExecCommand)
	api.Get("/servers/:id/history", commandHandler.GetHistory)
	api.Get("/commands/favorites", commandHandler.ListFavorites)
	api.Post("/commands/favorites/:id", commandHandler.ToggleFavorite)
	api.Delete("/commands/favorites/:id", commandHandler.DeleteFavorite)

	// Cron Jobs
	api.Get("/servers/:id/crons", cronHandler.ListCrons)
	api.Post("/servers/:id/crons", cronHandler.CreateCron)
	api.Put("/crons/:id", cronHandler.UpdateCron)
	api.Delete("/crons/:id", cronHandler.DeleteCron)
	api.Post("/crons/:id/run", cronHandler.RunCron)
	api.Post("/crons/:id/toggle", cronHandler.ToggleCron)
	api.Get("/crons/:id/logs", cronHandler.GetCronLogs)

	// Coolify Proxy
	coolify := api.Group("/coolify")
	coolify.Get("/apps", coolifyHandler.ListApps)
	coolify.Get("/apps/:uuid", coolifyHandler.GetApp)
	coolify.Post("/apps/:uuid/restart", coolifyHandler.RestartApp)
	coolify.Post("/apps/:uuid/deploy", coolifyHandler.DeployApp)
	coolify.Get("/apps/:uuid/logs", coolifyHandler.GetAppLogs)
	coolify.Get("/apps/:uuid/envs", coolifyHandler.GetAppEnvs)
	coolify.Put("/apps/:uuid/envs", coolifyHandler.UpdateAppEnvs)
	coolify.Get("/databases", coolifyHandler.ListDatabases)
	coolify.Get("/services", coolifyHandler.ListServices)
	coolify.Get("/deployments", coolifyHandler.ListDeployments)

	// Ops Integration
	ops := api.Group("/ops")
	ops.Get("/overview", opsHandler.Overview)
	ops.Get("/sre/events", opsHandler.SREEvents)
	ops.Get("/tickets", opsHandler.Tickets)
	ops.Get("/reviews", opsHandler.Reviews)

	// AI Assistant
	ai := api.Group("/ai")
	ai.Post("/chat", aiHandler.Chat)
	ai.Post("/stream", aiHandler.ChatStream)
	ai.Post("/execute", aiHandler.ExecuteAIAction)
	ai.Post("/analyze-logs", aiHandler.AnalyzeLogs)
	ai.Post("/suggest-fix", aiHandler.SuggestFix)
	ai.Get("/conversations", aiHandler.ListConversations)
	ai.Get("/conversations/:id", aiHandler.GetConversation)
	ai.Delete("/conversations/:id", aiHandler.DeleteConversation)
}
