package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/aaronromeo/swolegen/internal/strava"
	"github.com/gofiber/fiber/v2"
)

type fakeStravaClient struct {
	acts []strava.Activity
	err  error
}

func (f fakeStravaClient) GetRecentActivities(_ context.Context, sinceDays int) ([]strava.Activity, error) {
	return f.acts, f.err
}

func TestStravaRecentEndpoint(t *testing.T) {
	// Create a test Fiber app
	app := fiber.New()

	// By default, use a fake client to avoid real HTTP calls.
	savedFactory := newStravaClient
	newStravaClient = func(ts strava.TokenSource) stravaClient {
		return fakeStravaClient{acts: []strava.Activity{{Name: "Morning Run", Type: "Run"}}}
	}
	t.Cleanup(func() { newStravaClient = savedFactory })

	registerStravaOAuth(app)

	t.Run("no token provided - should suggest OAuth", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/strava/recent", nil)
		resp, err := app.Test(req)
		if err != nil {
			t.Fatalf("app.Test error: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusUnauthorized {
			t.Fatalf("expected status 401, got %d", resp.StatusCode)
		}

		var result map[string]any
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}
		if result["requires_oauth"] != true {
			t.Fatal("expected requires_oauth to be true")
		}
		if result["oauth_url"] != "/oauth/strava/start" {
			t.Fatalf("expected oauth_url to be '/oauth/strava/start', got %v", result["oauth_url"])
		}
	})

	t.Run("token in Authorization header", func(t *testing.T) {
		if os.Getenv("RUN_STRAVA_REAL") == "1" {
			// In real mode, restore factory to use the real client
			saved := newStravaClient
			newStravaClient = func(ts strava.TokenSource) stravaClient { return strava.NewWithTokenSource(ts) }
			t.Cleanup(func() { newStravaClient = saved })
		}

		req := httptest.NewRequest("GET", "/strava/recent", nil)
		req.Header.Set("Authorization", "Bearer test-token")
		resp, err := app.Test(req)
		if err != nil {
			t.Fatalf("app.Test error: %v", err)
		}
		defer resp.Body.Close()

		if os.Getenv("RUN_STRAVA_REAL") == "1" {
			// In real mode, token is invalid, expect 401
			if resp.StatusCode != http.StatusUnauthorized {
				t.Fatalf("expected status 401, got %d", resp.StatusCode)
			}
		} else {
			// In fake mode we should get 200 with stubbed activities
			if resp.StatusCode != http.StatusOK {
				t.Fatalf("expected status 200 with fake client, got %d", resp.StatusCode)
			}
			var result map[string]any
			if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
				t.Fatalf("failed to decode response: %v", err)
			}
			if result["count"] != float64(1) { // JSON numbers decode to float64
				t.Fatalf("expected count 1, got %v", result["count"])
			}
		}
	})

	t.Run("invalid Authorization header format", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/strava/recent", nil)
		req.Header.Set("Authorization", "InvalidFormat test-token")
		resp, err := app.Test(req)
		if err != nil {
			t.Fatalf("app.Test error: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusUnauthorized {
			t.Fatalf("expected status 401, got %d", resp.StatusCode)
		}
		var result map[string]any
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}
		if result["requires_oauth"] != true {
			t.Fatal("expected requires_oauth to be true for invalid auth format")
		}
	})

	t.Run("days parameter parsing", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/strava/recent?days=invalid", nil)
		resp, err := app.Test(req)
		if err != nil {
			t.Fatalf("app.Test error: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusUnauthorized {
			t.Fatalf("expected status 401, got %d", resp.StatusCode)
		}
	})
}
