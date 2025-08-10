package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"

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
		// Save token in process-global cache for immediate use by smoke endpoint
		strava.SetProcessToken(tok)
		// Return token JSON so user can persist if desired.
		c.Set("Content-Type", "application/json")
		enc := json.NewEncoder(c)
		enc.SetIndent("", "  ")
		return enc.Encode(tok)
	})

	// Smoke test endpoint: fetch recent activities via Strava client
	app.Get("/strava/recent", func(c *fiber.Ctx) error {
		daysStr := c.Query("days", "7")
		days, err := strconv.Atoi(daysStr)
		if err != nil || days < 0 {
			days = 7
		}
		cl := strava.NewWithTokenSource(strava.ProcessTokenSource{})
		acts, err := cl.GetRecentActivities(context.Background(), days)
		if err != nil {
			return c.Status(http.StatusInternalServerError).SendString(err.Error())
		}
		return c.JSON(struct {
			Count      int               `json:"count"`
			Activities []strava.Activity `json:"activities"`
		}{Count: len(acts), Activities: acts})
	})
}
