package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
)

func TestStravaRecentEndpoint(t *testing.T) {
	// Create a test Fiber app
	app := fiber.New()
	registerStravaOAuth(app)

	t.Run("no token provided - should suggest OAuth", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/strava/recent", nil)
		resp, err := app.Test(req)
		if err != nil {
			t.Fatalf("app.Test error: %v", err)
		}
		defer func() {
			err := resp.Body.Close()
			if err != nil {
				t.Fatalf("close response body error: %v", err)
			}
		}()

		if resp.StatusCode != http.StatusUnauthorized {
			t.Fatalf("expected status 401, got %d", resp.StatusCode)
		}

		var result map[string]interface{}
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
		req := httptest.NewRequest("GET", "/strava/recent", nil)
		req.Header.Set("Authorization", "Bearer test-token")

		resp, err := app.Test(req)
		if err != nil {
			t.Fatalf("app.Test error: %v", err)
		}
		defer func() {
			err := resp.Body.Close()
			if err != nil {
				t.Fatalf("close response body error: %v", err)
			}
		}()

		// Should get 401 because test-token is invalid, but should indicate token issue
		if resp.StatusCode != http.StatusUnauthorized {
			t.Fatalf("expected status 401, got %d", resp.StatusCode)
		}

		var result map[string]interface{}
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

	t.Run("invalid Authorization header format", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/strava/recent", nil)
		req.Header.Set("Authorization", "InvalidFormat test-token")

		resp, err := app.Test(req)
		if err != nil {
			t.Fatalf("app.Test error: %v", err)
		}
		defer func() {
			err := resp.Body.Close()
			if err != nil {
				t.Fatalf("close response body error: %v", err)
			}
		}()

		// Should get 401 and suggest OAuth (no valid Bearer token found)
		if resp.StatusCode != http.StatusUnauthorized {
			t.Fatalf("expected status 401, got %d", resp.StatusCode)
		}

		var result map[string]interface{}
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
		defer func() {
			err := resp.Body.Close()
			if err != nil {
				t.Fatalf("close response body error: %v", err)
			}
		}()

		// Should still process (defaults to 7 days) and return OAuth error
		if resp.StatusCode != http.StatusUnauthorized {
			t.Fatalf("expected status 401, got %d", resp.StatusCode)
		}
	})
}
