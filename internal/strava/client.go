package strava

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"sync/atomic"
	"time"

	"github.com/hashicorp/go-retryablehttp"
)

type TokenSource interface {
	Current(ctx context.Context) (*Token, error)
	Save(ctx context.Context, t *Token) error
}

// In-memory process-global token cache using atomic.Pointer (MVP single user).
var memTok atomic.Pointer[Token]

func SetProcessToken(t *Token) {
	if t == nil {
		memTok.Store(nil)
		return
	}
	cp := *t // store a copy to avoid external mutation
	memTok.Store(&cp)
}

func GetProcessToken() *Token {
	p := memTok.Load()
	if p == nil {
		return nil
	}
	cp := *p
	return &cp
}

// ProcessTokenSource returns/updates the in-memory process token only.
type ProcessTokenSource struct{}

func (ProcessTokenSource) Current(ctx context.Context) (*Token, error) {
	t := GetProcessToken()
	if t == nil {
		return nil, fmt.Errorf("no process token set; run OAuth handshake first")
	}
	return t, nil
}

func (ProcessTokenSource) Save(ctx context.Context, t *Token) error {
	if t != nil {
		SetProcessToken(t)
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
	Name   string  `json:"name"`
	Type   string  `json:"type"`
	Start  string  `json:"start_date"`
	Effort float64 `json:"suffer_score"` // Strava may return numbers with decimals
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
	_ = c.source.Save(ctx, tok) // best-effort (stores refreshed token in memory)

	// Build URL with sinceDays → after=unix seconds, per_page=100 (single page MVP)
	u, _ := url.Parse(activitiesURL)
	q := u.Query()
	q.Set("per_page", "100")
	if sinceDays > 0 {
		after := time.Now().Add(-time.Duration(sinceDays) * 24 * time.Hour).Unix()
		q.Set("after", strconv.FormatInt(after, 10))
	}
	u.RawQuery = q.Encode()

	req, _ := retryablehttp.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
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
