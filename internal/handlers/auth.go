package handlers

import (
	"log/slog"
	"strings"

	"github.com/ahmetk3436/bastion/internal/config"
	"github.com/ahmetk3436/bastion/internal/middleware"
	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

type AuthHandler struct {
	cfg          *config.Config
	passwordHash string
}

func NewAuthHandler(cfg *config.Config) *AuthHandler {
	// Hash the admin password on startup
	hash, err := bcrypt.GenerateFromPassword([]byte(cfg.AdminPassword), bcrypt.DefaultCost)
	if err != nil {
		slog.Error("Failed to hash admin password", "error", err)
	}
	return &AuthHandler{
		cfg:          cfg,
		passwordHash: string(hash),
	}
}

func (h *AuthHandler) Login(c *fiber.Ctx) error {
	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error":   true,
			"message": "Invalid request body",
		})
	}

	if req.Username != h.cfg.AdminUsername {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error":   true,
			"message": "Invalid credentials",
		})
	}

	if err := bcrypt.CompareHashAndPassword([]byte(h.passwordHash), []byte(req.Password)); err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error":   true,
			"message": "Invalid credentials",
		})
	}

	displayName := h.cfg.AdminDisplayName
	role := h.cfg.AdminRole

	access, refresh, err := middleware.GenerateTokens(req.Username, h.cfg.JWTSecret, displayName, role)
	if err != nil {
		slog.Error("Failed to generate tokens", "error", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error":   true,
			"message": "Failed to generate tokens",
		})
	}

	// Build avatar initials from display name
	initials := buildInitials(displayName)

	return c.JSON(fiber.Map{
		"access_token":  access,
		"refresh_token": refresh,
		"user": fiber.Map{
			"username":        req.Username,
			"display_name":    displayName,
			"role":            role,
			"avatar_initials": initials,
		},
	})
}

func (h *AuthHandler) Refresh(c *fiber.Ctx) error {
	var req struct {
		RefreshToken string `json:"refresh_token"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error":   true,
			"message": "Invalid request body",
		})
	}

	claims := &middleware.Claims{}
	token, err := jwt.ParseWithClaims(req.RefreshToken, claims, func(t *jwt.Token) (interface{}, error) {
		return []byte(h.cfg.JWTSecret), nil
	})

	if err != nil || !token.Valid {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error":   true,
			"message": "Invalid or expired refresh token",
		})
	}

	access, refresh, err := middleware.GenerateTokens(claims.Username, h.cfg.JWTSecret, claims.DisplayName, claims.Role)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error":   true,
			"message": "Failed to generate tokens",
		})
	}

	return c.JSON(fiber.Map{
		"access_token":  access,
		"refresh_token": refresh,
		"user": fiber.Map{
			"username":        claims.Username,
			"display_name":    claims.DisplayName,
			"role":            claims.Role,
			"avatar_initials": buildInitials(claims.DisplayName),
		},
	})
}

func (h *AuthHandler) Me(c *fiber.Ctx) error {
	username, _ := c.Locals("username").(string)
	displayName, _ := c.Locals("display_name").(string)
	role, _ := c.Locals("role").(string)

	return c.JSON(fiber.Map{
		"username":        username,
		"display_name":    displayName,
		"role":            role,
		"avatar_initials": buildInitials(displayName),
	})
}

func (h *AuthHandler) ChangePassword(c *fiber.Ctx) error {
	var req struct {
		OldPassword string `json:"old_password"`
		NewPassword string `json:"new_password"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error":   true,
			"message": "Invalid request body",
		})
	}

	if req.OldPassword == "" || req.NewPassword == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error":   true,
			"message": "Both old_password and new_password are required",
		})
	}

	if len(req.NewPassword) < 8 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error":   true,
			"message": "New password must be at least 8 characters",
		})
	}

	// Verify old password
	if err := bcrypt.CompareHashAndPassword([]byte(h.passwordHash), []byte(req.OldPassword)); err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error":   true,
			"message": "Current password is incorrect",
		})
	}

	// Hash new password
	newHash, err := bcrypt.GenerateFromPassword([]byte(req.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		slog.Error("Failed to hash new password", "error", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error":   true,
			"message": "Failed to update password",
		})
	}

	h.passwordHash = string(newHash)
	slog.Info("Admin password changed successfully")

	return c.JSON(fiber.Map{
		"message": "Password changed successfully",
	})
}

// buildInitials extracts uppercase initials from a display name.
// e.g. "Ahmet Kizilkaya" -> "AK", "Ahmet" -> "A"
func buildInitials(name string) string {
	if name == "" {
		return "?"
	}
	parts := strings.Fields(name)
	initials := ""
	for _, p := range parts {
		if len(p) > 0 {
			initials += strings.ToUpper(p[:1])
		}
	}
	if len(initials) > 2 {
		initials = initials[:2]
	}
	return initials
}
