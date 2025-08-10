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
	t        *testing.T
	status   int
	body     []byte
	lastURL  string
	sawAuth  string
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
	ts := fixedTokenSource{tok: &Token{AccessToken: "x", RefreshToken: "", ExpiresAt: time.Now().Add(365*24*time.Hour).Unix()}}
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
	u, _ := url.Parse(ft.lastURL)
	if u.Query().Get("after") == "" {
		t.Fatalf("expected 'after' query param in URL, got %q", ft.lastURL)
	}
}

func TestGetRecentActivities_HTTPError(t *testing.T) {
	ts := fixedTokenSource{tok: &Token{AccessToken: "x", ExpiresAt: time.Now().Add(365*24*time.Hour).Unix()}}
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

