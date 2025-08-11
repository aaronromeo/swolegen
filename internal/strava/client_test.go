package strava

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

type fixedTokenSource struct{ tok *Token }

func (f fixedTokenSource) Current(ctx context.Context) (*Token, error) { return f.tok, nil }
func (f fixedTokenSource) Save(ctx context.Context, t *Token) error    { return nil }

type fixtureTransport struct {
	t       *testing.T
	status  int
	body    []byte
	lastURL string
	sawAuth string
}

func (ft *fixtureTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	ft.lastURL = req.URL.String()
	ft.sawAuth = req.Header.Get("Authorization")
	resp := &http.Response{
		StatusCode: ft.status,
		Header:     make(http.Header),
		Body:       io.NopCloser(bytes.NewReader(ft.body)),
		Request:    req,
	}
	return resp, nil
}

func readFixture(t *testing.T, name string) []byte {
	t.Helper()
	p := filepath.Join("testdata", name)
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	return b
}

func TestGetRecentActivities_Success(t *testing.T) {
	// Prepare client with fixed token far in future (no refresh)
	ts := fixedTokenSource{tok: &Token{AccessToken: "x", RefreshToken: "", ExpiresAt: time.Now().Add(365 * 24 * time.Hour).Unix()}}
	c := NewWithTokenSource(ts)
	c.h.RetryMax = 0

	ft := &fixtureTransport{t: t, status: 200, body: readFixture(t, "activities.json")}
	c.h.HTTPClient.Transport = ft

	ctx := context.Background()
	acts, err := c.GetRecentActivities(ctx, 7)
	if err != nil {
		t.Fatalf("GetRecentActivities error: %v", err)
	}
	if len(acts) != 2 {
		t.Fatalf("expected 2 activities, got %d", len(acts))
	}
	if !strings.HasPrefix(ft.sawAuth, "Bearer ") {
		t.Fatalf("expected Authorization Bearer header, got %q", ft.sawAuth)
	}
	// sinceDays>0 should include `after` in query
	u, err := url.Parse(ft.lastURL)
	if err != nil {
		t.Fatalf("parse URL error: %v", err)
	}

	if u.Query().Get("after") == "" {
		t.Fatalf("expected 'after' query param in URL, got %q", ft.lastURL)
	}
}

func TestGetRecentActivities_HTTPError(t *testing.T) {
	ts := fixedTokenSource{tok: &Token{AccessToken: "x", ExpiresAt: time.Now().Add(365 * 24 * time.Hour).Unix()}}
	c := NewWithTokenSource(ts)
	c.h.RetryMax = 0

	ft := &fixtureTransport{t: t, status: 500, body: []byte(`{"error":"boom"}`)}
	c.h.HTTPClient.Transport = ft

	ctx := context.Background()
	_, err := c.GetRecentActivities(ctx, 0)
	if err == nil {
		t.Fatalf("expected error on HTTP 500")
	}
}

func TestUserTokenSource(t *testing.T) {
	// Test with valid token
	token := &Token{
		AccessToken:  "test-access-token",
		RefreshToken: "test-refresh-token",
		ExpiresAt:    time.Now().Add(time.Hour).Unix(),
	}

	ts := &UserTokenSource{Token: token}

	ctx := context.Background()
	current, err := ts.Current(ctx)
	if err != nil {
		t.Fatalf("Current() error: %v", err)
	}
	if current.AccessToken != "test-access-token" {
		t.Fatalf("expected access token 'test-access-token', got %q", current.AccessToken)
	}

	// Test Save updates the token
	newToken := &Token{
		AccessToken:  "new-access-token",
		RefreshToken: "new-refresh-token",
		ExpiresAt:    time.Now().Add(2 * time.Hour).Unix(),
	}

	err = ts.Save(ctx, newToken)
	if err != nil {
		t.Fatalf("Save() error: %v", err)
	}

	current, err = ts.Current(ctx)
	if err != nil {
		t.Fatalf("Current() after Save error: %v", err)
	}
	if current.AccessToken != "new-access-token" {
		t.Fatalf("expected updated access token 'new-access-token', got %q", current.AccessToken)
	}

	// Test with nil token
	ts = &UserTokenSource{Token: nil}
	_, err = ts.Current(ctx)
	if err == nil {
		t.Fatal("expected error with nil token")
	}
	if !strings.Contains(err.Error(), "no user token provided") {
		t.Fatalf("expected 'no user token provided' error, got %q", err.Error())
	}
}
