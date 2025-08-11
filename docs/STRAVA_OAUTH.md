# Strava OAuth Integration (MVP, Single User)

SwoleGen now uses **Strava OAuth (Authorization Code)** instead of a personal token.

## 1) Create a Strava API Application
- Visit: https://www.strava.com/settings/api
- Set **Application Name**: `swolegen`
- **Authorization Callback Domain**: your app domain (e.g., `swolegen.example.com`)
- Save **Client ID** and **Client Secret**

## 2) Configure Environment
Set the following Dokku env vars:
- `STRAVA_CLIENT_ID` – numeric client id
- `STRAVA_CLIENT_SECRET` – secret from Strava
- `STRAVA_REDIRECT_BASE_URL` – e.g., `https://swolegen.example.com` (no trailing slash)
- `STRAVA_SCOPES` – default: `read,activity:read_all`
- `STRAVA_STATE_SECRET` – random 32+ char string (used to sign the OAuth state)
- `OPENAI_API_KEY` – already required
```
dokku config:set swolegen   STRAVA_CLIENT_ID=12345   STRAVA_CLIENT_SECRET=***   STRAVA_REDIRECT_BASE_URL=https://swolegen.example.com   STRAVA_SCOPES="read,activity:read_all"   STRAVA_STATE_SECRET="$(openssl rand -hex 32)"
```

## 3) Run the Handshake
1. Open: `https://swolegen.example.com/oauth/strava/start`
2. Approve on Strava
3. You’ll be redirected back to `/oauth/strava/callback`
4. The server will **print the token JSON in logs** and **return token JSON in the browser** (MVP)
5. Copy the `access_token`, `refresh_token`, and `expires_at` into Dokku env:
```
dokku config:set swolegen STRAVA_ACCESS_TOKEN=... STRAVA_REFRESH_TOKEN=... STRAVA_EXPIRES_AT=...
```
> For single-user MVP we avoid a DB. Tokens live in env. The server will **auto‑refresh** into memory; when it refreshes, it will print the updated tokens so you can update Dokku env later (optional).

## 4) How it Works (MVP)
- `GET /oauth/strava/start` – builds the authorize URL and redirects.
- `GET /oauth/strava/callback` – exchanges `code` for tokens, prints JSON, returns it.
- `GET /strava/recent` – fetches recent activities with optional user token support.
- `internal/strava/oauth.go` – helpers for building URLs, exchanging, and refreshing.
- `internal/strava/client.go` – uses `TokenSource` to inject a fresh Bearer token.

## 5) Using the `/strava/recent` Endpoint

The `/strava/recent` endpoint now supports user-specific access tokens via the standard OAuth 2.0 Authorization header:

### Usage (OAuth 2.0 Standard)
```bash
curl -H "Authorization: Bearer YOUR_ACCESS_TOKEN" \
     "https://swolegen.example.com/strava/recent?days=7"
```

### Token Management Architecture
- **Frontend responsibility**: Store complete token (access + refresh + expiry)
- **Frontend responsibility**: Check token expiry before API calls
- **Frontend responsibility**: Refresh tokens when needed using `/oauth/strava/callback` flow
- **Backend responsibility**: Accept valid access tokens via Authorization header
- **Backend responsibility**: Return 401 when tokens are invalid/expired

### Token Refresh Flow (Frontend-Managed)
1. Frontend checks if access token expires soon (< 5 minutes)
2. If expiring, frontend calls Strava token refresh endpoint directly
3. Frontend updates stored token with new values
4. Frontend makes API call with fresh access token

### Response Format

**Success:**
```json
{
  "count": 5,
  "activities": [
    {
      "name": "Morning Run",
      "type": "Run",
      "start_date": "2024-01-15T06:30:00Z",
      "suffer_score": 45.0
    }
  ]
}
```

**Token expired/invalid:**
```json
{
  "error": "Token invalid or expired",
  "oauth_url": "/oauth/strava/start",
  "message": "Please re-authenticate with Strava",
  "requires_oauth": true
}
```

### Frontend Token Caching Strategy

1. **Store tokens in frontend** (localStorage, sessionStorage, or secure cookie)
2. **Include token in API calls** using Authorization header
3. **Handle token refresh** by checking the returned `token` field
4. **Redirect to OAuth** when `requires_oauth: true` is returned

## 6) Security Notes
- State parameter is HMAC-signed with `STRAVA_STATE_SECRET` and includes a timestamp to mitigate CSRF/replay.
- Single-user assumption: token is stored in env; no user table/sessions.
- For multi-user later, store tokens per user (DB) and tie `state` to a user session.

