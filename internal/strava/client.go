package strava

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/hashicorp/go-retryablehttp"
)

type TokenSource interface {
	Current(ctx context.Context) (*Token, error)
	Save(ctx context.Context, t *Token) error
}

// UserTokenSource uses a user-provided token. Frontend is responsible for
// token refresh and management. Backend only validates the provided access token.
type UserTokenSource struct {
	Token *Token
}

func (u *UserTokenSource) Current(ctx context.Context) (*Token, error) {
	if u.Token == nil {
		return nil, fmt.Errorf("no user token provided; OAuth handshake required")
	}
	return u.Token, nil
}

func (u *UserTokenSource) Save(ctx context.Context, t *Token) error {
	// Update the token in memory so refreshed tokens are available
	if t != nil {
		u.Token = t
	}
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
	Name        string        `json:"name"`
	Type        string        `json:"type"`
	Start       string        `json:"start_date"`
	ElapsedTime time.Duration `json:"elapsed_time"`
	Effort      float64       `json:"suffer_score"` // Strava may return numbers with decimals
}

func (c *Client) GetRecentActivities(ctx context.Context, sinceDays int) ([]Activity, error) {
	tok, err := c.source.Current(ctx)
	if err != nil {
		return nil, err
	}

	// Build URL with sinceDays → after=unix seconds, per_page=100 (single page MVP)
	u, err := url.Parse(activitiesURL)
	if err != nil {
		return nil, err
	}

	q := u.Query()
	q.Set("per_page", "100")
	if sinceDays > 0 {
		after := time.Now().Add(-time.Duration(sinceDays) * 24 * time.Hour).Unix()
		q.Set("after", strconv.FormatInt(after, 10))
	}
	u.RawQuery = q.Encode()

	req, err := retryablehttp.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+tok.AccessToken)

	resp, err := c.h.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("strava status %d", resp.StatusCode)
	}
	var out []Activity
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	return out, nil
}
