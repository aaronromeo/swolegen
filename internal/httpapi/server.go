package httpapi

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/aaronromeo/swolegen/internal/config"
	"github.com/gofiber/fiber/v2"
)

func NewServer(cfg *config.Config, logger *slog.Logger) *fiber.App {
	app := fiber.New(
		fiber.Config{
			DisableStartupMessage: true,
			ReadTimeout:           30 * time.Second,
			WriteTimeout:          60 * time.Second,
		},
	)
	app.Get("/healthz", func(c *fiber.Ctx) error { return c.SendStatus(http.StatusOK) })
	registerStravaOAuth(app, cfg)
	registerLLM(app, cfg, logger)
	// Serve a very basic frontend to exercise the OAuth flow and recent activities
	app.Static("/", "./web")
	return app
}
