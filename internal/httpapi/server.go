package httpapi

import (
	"net/http"
	"time"

	"github.com/gofiber/fiber/v2"
)

func NewServer() *fiber.App {
	app := fiber.New(fiber.Config{DisableStartupMessage: true, ReadTimeout: 30 * time.Second, WriteTimeout: 60 * time.Second})
	app.Get("/healthz", func(c *fiber.Ctx) error { return c.SendStatus(http.StatusOK) })
	registerStravaOAuth(app)
	// Serve a very basic frontend to exercise the OAuth flow and recent activities
	app.Static("/", "./web")
	return app
}
