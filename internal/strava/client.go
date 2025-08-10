package strava

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/hashicorp/go-retryablehttp"
)

type TokenSource interface {
	Current(ctx context.Context) (*Token, error)
	Save(ctx context.Context, t *Token) error
}

// EnvTokenSource is a single-user MVP token source stored in env vars.
type EnvTokenSource struct{}

func (EnvTokenSource) Current(ctx context.Context) (*Token, error) {
	// In MVP, read from env vars if present. Caller may overwrite in memory after refresh.
	// We keep it simple: the httpapi layer can keep a process-global cached token.
	return nil, fmt.Errorf("EnvTokenSource.Current not implemented (wire your storage)")
}
func (EnvTokenSource) Save(ctx context.Context, t *Token) error {
	// MVP: no-op; you may print to logs for manual env update.
	return nil
}

type Client struct {
	h      *retryablehttp.Client
	source TokenSource
}

func NewWithTokenSource(ts TokenSource) *Client {
	h := retryablehttp.NewClient()
	h.RetryMax = 3
	return &Client{h: h, source: ts}
}

type Activity struct {
	Name   string `json:"name"`
	Type   string `json:"type"`
	Start  string `json:"start_date"`
	Effort int    `json:"suffer_score"` // may be missing for some accounts
}

func (c *Client) GetRecentActivities(ctx context.Context, sinceDays int) ([]Activity, error) {
	tok, err := c.source.Current(ctx)
	if err != nil {
		return nil, err
	}
	tok, err = RefreshIfNeeded(ctx, tok)
	if err != nil {
		return nil, err
	}
	_ = c.source.Save(ctx, tok) // best-effort

	req, _ := retryablehttp.NewRequestWithContext(ctx, http.MethodGet, "https://www.strava.com/api/v3/athlete/activities", nil)
	req.Header.Set("Authorization", "Bearer "+tok.AccessToken)
	resp, err := c.h.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("strava status %d", resp.StatusCode)
	}
	var out []Activity
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	return out, nil
}
