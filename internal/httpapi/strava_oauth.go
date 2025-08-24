package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"github.com/aaronromeo/swolegen/internal/config"
	"github.com/aaronromeo/swolegen/internal/strava"
	"github.com/gofiber/fiber/v2"
)

type stravaClient interface {
	GetRecentActivities(ctx context.Context, sinceDays int) ([]strava.Activity, error)
}

// newStravaClient is a factory for creating Strava clients. Tests may override this
// to inject a client with a stubbed implementation.
var newStravaClient = func(ts strava.TokenSource) stravaClient { return strava.NewWithTokenSource(ts) }

// OAuth endpoints for Strava (MVP single user).

func registerStravaOAuth(app *fiber.App, _ *config.Config) {
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
		// Return token JSON so user can persist if desired.
		c.Set("Content-Type", "application/json")
		enc := json.NewEncoder(c)
		enc.SetIndent("", "  ")
		return enc.Encode(tok)
	})

	// Fetch recent activities with optional user token (GET and POST)
	stravaRecentHandler := func(c *fiber.Ctx) error {
		daysStr := c.Query("days", "7")
		days, err := strconv.Atoi(daysStr)
		if err != nil || days < 0 {
			days = 7
		}

		// Check for user-provided token in Authorization header (OAuth 2.0 standard)
		var userToken *strava.Token
		authHeader := c.Get("Authorization")
		if authHeader != "" && strings.HasPrefix(authHeader, "Bearer ") {
			accessToken := strings.TrimPrefix(authHeader, "Bearer ")
			userToken = &strava.Token{AccessToken: accessToken}
		}

		var cl stravaClient
		var tokenSource strava.TokenSource

		if userToken != nil {
			// Try with user-provided token
			tokenSource = &strava.UserTokenSource{Token: userToken}
			cl = newStravaClient(tokenSource)
		} else {
			return c.Status(http.StatusUnauthorized).JSON(fiber.Map{
				"error":          "No user token provided; OAuth handshake required",
				"oauth_url":      "/oauth/strava/start",
				"message":        "Please authenticate with Strava first",
				"requires_oauth": true,
			})
		}

		acts, err := cl.GetRecentActivities(context.Background(), days)
		if err != nil {
			return c.Status(http.StatusUnauthorized).JSON(fiber.Map{
				"error":          err.Error(),
				"oauth_url":      "/oauth/strava/start",
				"message":        "Please authenticate with Strava first",
				"requires_oauth": true,
			})
		}

		return c.JSON(fiber.Map{
			"count":      len(acts),
			"activities": acts,
		})
	}

	// Register only GET - this is a data retrieval endpoint
	app.Get("/strava/recent", stravaRecentHandler)
}
