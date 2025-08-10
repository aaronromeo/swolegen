package httpapi

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/aaronromeo/swolegen/internal/strava"
	"github.com/gofiber/fiber/v2"
)

// OAuth endpoints for Strava (MVP single user).

func registerStravaOAuth(app *fiber.App) {
	app.Get("/oauth/strava/start", func(c *fiber.Ctx) error {
		u, err := strava.AuthorizeURL()
		if err != nil {
			return c.Status(http.StatusInternalServerError).SendString(err.Error())
		}
		return c.Redirect(u, http.StatusFound)
	})

	app.Get("/oauth/strava/callback", func(c *fiber.Ctx) error {
		state := c.Query("state")
		if err := strava.ValidateState(state); err != nil {
			return c.Status(http.StatusBadRequest).SendString("invalid state")
		}
		code := c.Query("code")
		if code == "" {
			return c.Status(http.StatusBadRequest).SendString("missing code")
		}
		tok, err := strava.ExchangeCode(context.Background(), code)
		if err != nil {
			return c.Status(http.StatusInternalServerError).SendString(err.Error())
		}
		// Return token JSON (and print in logs upstream) so user can store in env if desired.
		c.Set("Content-Type", "application/json")
		enc := json.NewEncoder(c)
		enc.SetIndent("", "  ")
		return enc.Encode(tok)
	})
}
