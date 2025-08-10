package strava

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/hashicorp/go-retryablehttp"
)

type Token struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresAt    int64  `json:"expires_at"` // unix seconds
	TokenType    string `json:"token_type"`
	Scope        string `json:"scope"`
}

func scopes() string {
	s := os.Getenv("STRAVA_SCOPES")
	if s == "" {
		return "read,activity:read_all"
	}
	return s
}

func redirectBase() (string, error) {
	base := os.Getenv("STRAVA_REDIRECT_BASE_URL")
	if base == "" {
		return "", fmt.Errorf("STRAVA_REDIRECT_BASE_URL not set")
	}
	return strings.TrimRight(base, "/"), nil
}

func clientID() (string, error) {
	id := os.Getenv("STRAVA_CLIENT_ID")
	if id == "" {
		return "", fmt.Errorf("STRAVA_CLIENT_ID not set")
	}
	return id, nil
}

func clientSecret() (string, error) {
	sec := os.Getenv("STRAVA_CLIENT_SECRET")
	if sec == "" {
		return "", fmt.Errorf("STRAVA_CLIENT_SECRET not set")
	}
	return sec, nil
}

func stateSecret() ([]byte, error) {
	sec := os.Getenv("STRAVA_STATE_SECRET")
	if sec == "" {
		return nil, fmt.Errorf("STRAVA_STATE_SECRET not set")
	}
	return []byte(sec), nil
}

// SignedState creates a short-lived HMAC'd state token.
func SignedState() (string, error) {
	key, err := stateSecret()
	if err != nil {
		return "", err
	}
	ts := time.Now().Unix()
	msg := fmt.Sprintf("%d", ts)
	mac := hmac.New(sha256.New, key)
	mac.Write([]byte(msg))
	sig := mac.Sum(nil)
	raw := fmt.Sprintf("%d.%s", ts, base64.RawURLEncoding.EncodeToString(sig))
	return raw, nil
}

// ValidateState checks HMAC and age (5 minutes).
func ValidateState(raw string) error {
	key, err := stateSecret()
	if err != nil {
		return err
	}
	parts := strings.Split(raw, ".")
	if len(parts) != 2 {
		return fmt.Errorf("bad state format")
	}
	tsStr, sigB64 := parts[0], parts[1]
	ts, err := strconv.ParseInt(tsStr, 10, 64)
	if err != nil {
		return fmt.Errorf("bad state ts")
	}
	if time.Since(time.Unix(ts, 0)) > 5*time.Minute {
		return fmt.Errorf("state expired")
	}
	mac := hmac.New(sha256.New, key)
	mac.Write([]byte(tsStr))
	expected := mac.Sum(nil)
	got, err := base64.RawURLEncoding.DecodeString(sigB64)
	if err != nil {
		return fmt.Errorf("state b64")
	}
	if !hmac.Equal(expected, got) {
		return fmt.Errorf("state mismatch")
	}
	return nil
}

func AuthorizeURL() (string, error) {
	cbBase, err := redirectBase()
	if err != nil {
		return "", err
	}
	cid, err := clientID()
	if err != nil {
		return "", err
	}
	state, err := SignedState()
	if err != nil {
		return "", err
	}

	u, _ := url.Parse(authURL)
	q := u.Query()
	q.Set("client_id", cid)
	q.Set("response_type", "code")
	q.Set("redirect_uri", cbBase+"/oauth/strava/callback")
	q.Set("approval_prompt", "auto")
	q.Set("scope", scopes())
	q.Set("state", state)
	u.RawQuery = q.Encode()
	return u.String(), nil
}

// ExchangeCode for tokens
func ExchangeCode(ctx context.Context, code string) (*Token, error) {
	cid, err := clientID()
	if err != nil {
		return nil, err
	}
	sec, err := clientSecret()
	if err != nil {
		return nil, err
	}

	h := retryablehttp.NewClient()
	h.RetryMax = 2

	vals := url.Values{}
	vals.Set("client_id", cid)
	vals.Set("client_secret", sec)
	vals.Set("code", code)
	vals.Set("grant_type", "authorization_code")

	req, _ := retryablehttp.NewRequestWithContext(ctx, "POST", tokenURL, strings.NewReader(vals.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := h.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("token exchange status %d", resp.StatusCode)
	}
	var t Token
	if err := json.NewDecoder(resp.Body).Decode(&t); err != nil {
		return nil, err
	}
	return &t, nil
}

// RefreshIfNeeded refreshes using refresh_token when expired or near expiry (<=120s).
func RefreshIfNeeded(ctx context.Context, tok *Token) (*Token, error) {
	if tok == nil {
		return nil, fmt.Errorf("nil token")
	}
	if time.Until(time.Unix(tok.ExpiresAt, 0)) > 120*time.Second {
		return tok, nil
	}

	cid, err := clientID()
	if err != nil {
		return nil, err
	}
	sec, err := clientSecret()
	if err != nil {
		return nil, err
	}

	h := retryablehttp.NewClient()
	h.RetryMax = 2

	vals := url.Values{}
	vals.Set("client_id", cid)
	vals.Set("client_secret", sec)
	vals.Set("grant_type", "refresh_token")
	vals.Set("refresh_token", tok.RefreshToken)

	req, _ := retryablehttp.NewRequestWithContext(ctx, "POST", tokenURL, strings.NewReader(vals.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := h.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("refresh status %d", resp.StatusCode)
	}
	var t Token
	if err := json.NewDecoder(resp.Body).Decode(&t); err != nil {
		return nil, err
	}
	return &t, nil
}
