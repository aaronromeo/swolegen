# SwoleGen — MVP Requirements (Single-Session, Fully Automatic)

## Overview

SwoleGen is a single-session weightlifting workout generator that builds a complete, time-boxed workout in YAML format.
It consumes user rules, history, recent and upcoming cardio activity, available equipment, and optional recovery data to produce a workout the user can follow and log without editing YAML structure.

The YAML output is:

- **Human-readable** for in-workout use.
- **Pre-enumerated per set** so the user only fills in `actual_weight`, `actual_reps`, and optional `notes`.
- **Machine-parseable** for round-trip logging and analytics.

---

## Inputs (per request)

- `instructions_url` – user’s long-form rules (goals, bans, preferences).
- `history_url` – set-level history (load × reps × RIR/RPE, time/date, exercise name).
- `strava_recent` – last 7–14 days activities with Relative Effort (or equivalent load).
- `upcoming_cardio_text` – short free-text list of planned cardio next 2–4 days.
- `location` – string key: `gym:<name>` / `home` / `hotel:<name>`.
- `equipment_inventory` – list of human names (e.g., `"barbell"`, `"db_set_5–100"`, `"cables"`, `"sled"`, `"kb_pair_45"`, `"pullup_bar"`, `"bands"`).
- `duration_minutes` – integer (e.g., 30, 45, 60).
- `units` – `"lbs"` or `"kg"` (default `"lbs"`).
- Optional recovery signals:
  - `garmin_sleep_score` (0–100)
  - `garmin_body_battery` (0–100)

---

## Core Logic

### 1. Session Type Selection
- Use last 2–3 logged strength sessions to avoid repeats.
- Example: if last were Arm → Pull, bias Push/Lower/etc.
- Allow repeats if last workout was ≥7 days ago.
- Respect instruction bans (e.g., “no barbell OHP”, “RDL-only”).

### 2. Fatigue Gating
- Compute fatigue index:
  - Rolling Strava Relative Effort (7d)
  - Penalty for hard upcoming cardio within 24–48h.
- If `garmin_sleep_score < 60` or `body_battery < 40`:
  - Cut Tier C
  - Increase RIR by +1
  - Cap top-set load at ≤95% of recent best.

### 3. Equipment Routing
- Only choose movements available in `equipment_inventory`.
- Provide `swap_rules` for common substitutes (barbell → dumbbell, cable → band).

### 4. Timeboxing
- Build workout with Warm-up (`W`), Tier A (must), Tier B (good to do), and Tier C (optional).
- Honor `duration_minutes` by estimating per-set time and pushing overflow to Tier B then Tier C.
- Output `cut_order: ["C","B"]`.

### 5. Progression
- **Double progression (default)**:
  - Keep load fixed until all sets hit the top of the rep range at target RIR/RPE, then increase load next time.
  - Example: 3×8–12 @ RIR 2 → if 12/12/12, add ~2–5% next time.
- Skip `%1RM` for MVP.
- Estimate `e1RM` from history for later load guidance.
- Apply ±5–10% guardrails to target load from recent best.

### 6. Safety
- Cap `target_load` ≤ recent best × 1.05 (or lower if fatigued).
- Respect injury flags/bans from inputs.
- Default RIR: 1–3 for compounds; 0 RIR only if explicitly allowed.

### 7. Supersets / Rest
- Allowed. Use `superset` field for grouping (e.g., `"A1"`, `"B1"`).
- Rest is time-based per prescription.
- Heart-rate is not an input for MVP.

---

## Output Format — Workout YAML v1.2

- **One node per set** — no editing structure during workout.
- Pre-filled with target reps, load, RIR, rest.
- User fills only `actual_weight`, `actual_reps`, `notes`.

### IDs
- `workout_id`: `YYYY-MM-DD-<kebab-location>-NN` (NN = deterministic 2-digit seed from inputs).
- `set_id`: `<TIER>-<SLUG>-(WU#|#)` (e.g., `A-RDL-WU1`, `B-DBIP-3`).
- Stable IDs → safe round-trip logging, merge, and analytics.

### Schema Highlights
- `tier`: "W" | "A" | "B" | "C"
- `must`: `true` for W/A; `false` for B/C.
- `superset`: grouping label or `null`.
- `target_reps`: integer or string range (`"8-12"`, `"20/side"`).
- `target_weight`: number or `null`.
- `rir`: integer or `null`.
- `rest_s`: integer seconds.

A full JSON Schema for validation is defined in [`schemas/workout-v1.2.json`](schemas/workout-v1.2.json).

---

## Processing Flow

1. **Analyzer (Step 1)**
   - Input: all user data (instructions, history, Strava, recovery, equipment, etc.).
   - Output: compact JSON plan ([`schemas/analyzer-v1.json`](schemas/analyzer-v1.json)) selecting session type, tiers, fatigue policy, time budget, and per-exercise set counts/targets.
   - Uses 90d history for recent bests, 14d for anti-repeat.

2. **Generator (Step 2)**
   - Input: Analyzer JSON.
   - Output: Workout YAML v1.2 (one node per set).
   - Applies fatigue policy to loads/RIR.
   - Pre-enumerates warm-ups and working sets.

3. **Validation**
   - Both Analyzer JSON and Workout YAML are validated against their schemas.
   - If invalid, the generator is re-prompted with error list to self-heal.

---

## Validation

- **Analyzer JSON Schema**: [`schemas/analyzer-v1.json`](schemas/analyzer-v1.json)
- **Workout YAML Schema**: [`schemas/workout-v1.2.json`](schemas/workout-v1.2.json)
- On failure, re-prompt the LLM with validation errors and require a corrected re-emission.

---

## Example Workflow

1. App gathers:
   - `instructions_url`, `history_url`, `strava_recent`, `upcoming_cardio_text`, `location`, `equipment_inventory`, `duration_minutes`, `units`, `garmin_sleep_score`, `garmin_body_battery`.
2. App resolves URLs → passes full text/JSON to Analyzer.
3. Analyzer produces `analyzer-v1.json`.
4. Generator consumes Analyzer output → emits Workout YAML v1.2.
5. User logs workout by filling `actual_weight` and `actual_reps` in YAML.
6. App ingests updated YAML to update history.

---

## Quick Start (MVP)

### Run the API locally

1. Clone repo
   ```bash
   git clone https://github.com/aaronromeo/swolegen.git
   cd swolegen
   ```

2. Create and fill your env file
   ```bash
   cp .env.local .env
   # Edit .env and set values, at minimum:
   # STRAVA_CLIENT_ID=...
   # STRAVA_CLIENT_SECRET=...
   # STRAVA_REDIRECT_BASE_URL=http(s)://localhost:8080 (or your ngrok URL)
   # STRAVA_SCOPES=read,activity:read_all
   # STRAVA_STATE_SECRET=$(openssl rand -hex 32)
   # OPENAI_API_KEY=...   # only needed for analyzer/generator flows
   ```

3. (Optional) Source the .env into your shell for this terminal session
   - POSIX-safe approach (exports all keys):
     ```bash
     set -a; source .env; set +a
     ```
   - Alternative one-liner:
     ```bash
     export $(grep -v '^#' .env | xargs)
     ```

4. Build the server binary
   ```bash
   make build
   # or: go build -o build/swolegen-api ./cmd/swolegen-api
   ```

5. Run the API (defaults to :8080). To source env vars inline on the CLI, prefix the command:
   ```bash
   # Using values from .env inline (Bash/Zsh):
   set -a; source .env; set +a; ./build/swolegen-api

   # Or prefix only the variables you want to override:
   ADDR=:8080 ./build/swolegen-api
   ```

6. Validate via demo UI (preferred)
   - Open http://localhost:8080/ (or https://<NGROK_URL>/ if tunneling with ngrok)
   - Click "Start Strava OAuth" and approve on Strava
   - After redirect back to `/oauth/strava/callback`, copy the `access_token` from the JSON
   - Paste it into the "Access Token" field on the demo page
   - Set Days and click "Fetch /strava/recent" to see your recent activities

   Liveness check:
   ```bash
   curl -sSf http://localhost:8080/healthz -I
   ```

Next: for a detailed walkthrough, see “Quick Start — Test the Strava API (Preferred)” below.

### Generate via prompts (optional)

3. Call the analyzer
   ```bash
   openai api chat.completions.create -m gpt-4o -g prompts/analyzer-system.md -u prompts/analyzer-user.md
   ```

4. Call the generator
   ```bash
   openai api chat.completions.create -m gpt-4o -g prompts/generator-system.md -u prompts/generator-user.md
   ```

5. Validate the output
   ```bash
   ajv validate -s schemas/workout-v1.2.json -d output.yaml
   ```

### Quick Start — Test the Strava API (Preferred)

Use the built-in demo UI at `/` to drive the OAuth flow and test `/strava/recent`.

Prereqs:
- ngrok installed and authenticated (for public callback)
- Your backend running locally on http://localhost:8080

1) Start ngrok on port 8080 and copy the HTTPS URL
```bash
ngrok http 8080
```

2) Add required Strava env vars in `.env` and restart the server
```bash
STRAVA_CLIENT_ID=<your_client_id>
STRAVA_CLIENT_SECRET=<your_client_secret>
STRAVA_REDIRECT_BASE_URL=https://<NGROK_URL>   # no trailing slash
STRAVA_SCOPES=read,activity:read_all
STRAVA_STATE_SECRET=$(openssl rand -hex 32)
```

3) Configure your Strava app
- Authorization Callback Domain: your ngrok host (e.g., `xxxxx.ngrok-free.app`)
- If asked for a redirect URI, use: `https://<NGROK_URL>/oauth/strava/callback`

4) Use the demo UI
- Open `https://<NGROK_URL>/` (or `http://localhost:8080/` if testing locally without Strava redirect)
- Click "Start Strava OAuth" and approve
- After redirect to `/oauth/strava/callback`, copy the `access_token` from the JSON
- Paste it into the "Access Token" field on the demo page
- Set Days and click "Fetch /strava/recent" to see activities

CLI alternative
```bash
curl -v -H "Authorization: Bearer <ACCESS_TOKEN>" \
     "https://<NGROK_URL>/strava/recent?days=7" | jq
```

Notes/Troubleshooting:
- 401/403: re-run OAuth or check token expiry; ensure the Authorization header is present.
- Redirect mismatch: `STRAVA_REDIRECT_BASE_URL` must match the current ngrok URL and Strava app settings.
- Scopes: ensure `STRAVA_SCOPES` includes `read,activity:read_all`.
- Token management: Frontend owns token storage/refresh; backend only performs OAuth handshake and validates Bearer tokens on `/strava/recent`.

See also: `docs/STRAVA_OAUTH.md`.

---

4) Run the OAuth handshake:
- Open in your browser: `https://<NGROK_URL>/oauth/strava/start`
- Approve on Strava; you'll be redirected to `/oauth/strava/callback`
- Copy the `access_token` from the JSON shown

5) Verify the recent-activities endpoint with your token:

```bash
curl -v -H "Authorization: Bearer <ACCESS_TOKEN>" \
     "https://<NGROK_URL>/strava/recent?days=7" | jq
```

Notes/Troubleshooting:
- 401/403: redo step 4 or check token expiry; ensure Authorization header is present.
- Redirect mismatch: ensure `STRAVA_REDIRECT_BASE_URL` matches the current ngrok URL and Strava app settings.
- Scopes: confirm `STRAVA_SCOPES` includes `read,activity:read_all`.
- Token management: Frontend owns token storage/refresh. Backend only handles the OAuth handshake (`/oauth/strava/start` → `/oauth/strava/callback`) and validates the Bearer token on `/strava/recent`. Do not persist tokens server-side.

See also: `docs/STRAVA_OAUTH.md`.

---

## Schemas

Validation is critical for ensuring generated plans are consistent and machine-parseable.

- **Analyzer JSON Schema:** `schemas/analyzer-v1.json`
- **Workout YAML Schema:** `schemas/workout-v1.2.json`

You can validate using:

```bash
# JSON validation
ajv validate -s schemas/analyzer-v1.json -d examples/analyzer-v1.example.json

# YAML validation (convert to JSON first)
yq -o=json eval examples/workout-v1.2.example.yaml |   ajv validate -s schemas/workout-v1.2.json -d /dev/stdin
```

---

## ID Generation Rules

- **workout_id**: `YYYY-MM-DD-<kebab-location>-NN`
  NN is a deterministic 2-digit seed from:
  - goal
  - duration
  - sorted equipment list
  - last-14d session pattern

- **set_id**: `<TIER>-<SLUG>-(WU#|#)`
  - Slug is a short, uppercase identifier from the exercise name.
  - Examples: `A-RDL-1`, `B-DBIP-3`, `A-RDL-WU1`.

Stable IDs allow safe round-trip logging, merge safety, and historical analytics.

---

## Repo Structure Recommendation

```
swolegen/
├── README.md
├── prompts/
│   ├── analyzer-system.md
│   ├── analyzer-user.md
│   ├── generator-system.md
│   ├── generator-user.md
│   ├── repair-prompt.md
├── schemas/
│   ├── workout-v1.2.json
│   └── analyzer-v1.json
├── examples/
│   ├── workout-v1.2.example.yaml
│   └── analyzer-v1.example.json
└── LICENSE
```

**Why:**
- Prompts are modular and versionable.
- Schemas are explicit and testable.
- Examples double as fixtures for CI validation.

---

## References

- [Output YAML Gist](docs/sample-output.yaml)
- [JSON Schema: Workout](schemas/workout-v1.2.json)
- [JSON Schema: Analyzer](schemas/analyzer-v1.json)
