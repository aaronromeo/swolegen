# SwoleGen – Build Plan for Augment/Cursor

This backlog is ordered so each task is small, single‑purpose, and verifiable with unit tests. Copy tasks one‑by‑one into Augment/Cursor.

## 0. Repo hygiene
- [X] Create folders: `cmd/swolegen-api`, `internal/{httpapi,history,workout,id,llm,strava,schema}`, `prompts`, `schemas`, `examples`.
- [X] Add Go toolchain: `go 1.24.5` in `go.mod`.
- [X] LICENSE and basic README remain as-is.

## 1. Module init & dependencies
- [X] `go mod init github.com/aaronromeo/swolegen`
- [X] Add deps: `github.com/gofiber/fiber/v2`, `github.com/xeipuuv/gojsonschema`, `github.com/hashicorp/go-retryablehttp`, `github.com/cespare/xxhash/v2`.
- [X] `make lint` and `make test` targets (optional).

## 2. Config & env
- [ ] Implement minimal config loader (env only). Variables:
  - `OPENAI_API_KEY` (required)
  - `STRAVA_ACCESS_TOKEN` (optional for MVP if not using OAuth)
  - `TIMEZONE` default `America/Toronto`
  - `LLM_MODEL_ANALYZER` (default `gpt-4o-mini`)
  - `LLM_MODEL_GENERATOR` (default `gpt-4o`)
  - `LLM_MAX_TOKENS_ANALYZER` (default `2000`)
  - `LLM_MAX_TOKENS_GENERATOR` (default `2000`)
  - `LLM_RETRIES` (default `3`)
  - `STRAVA_TIMEOUT_S` (default `30`), `LLM_ANALYZER_TIMEOUT_S` (default `90`), `LLM_GENERATOR_TIMEOUT_S` (default `60`)
- [ ] Unit test: missing required env leads to clean error.

## 3. Schema embedding & validator
- [X] Place `schemas/analyzer-v1.json` and `schemas/workout-v1.2.json` in repo.
- [ ] Implement `internal/schema` with:
  - `func ValidateAnalyzerJSON(b []byte) error`
  - `func ValidateWorkoutYAML(b []byte) error` (convert YAML→JSON internally).
- [ ] Unit tests using `examples/*` fixtures (positive + negative cases).

## 4. ID + slug package
- [X] `internal/id`:
  - `Slug(exerciseName string) string` → `A–Z0–9–`, uppercased, max 12 chars.
  - `WorkoutID(date, location string, seedInput []byte) string` → `YYYY-MM-DD-<kebab-location>-NN` where `NN` is `xxhash(seedInput)%100` formatted `02d`.
  - `SetID(tier, slug string, n int, warmup bool) string` → e.g., `A-RDL-WU1`, `A-RDL-1`.
- [X] Unit tests for determinism and formatting.

## 5. History parsing (public URLs)
- [ ] `internal/history`:
  - `FetchURL(ctx, url string) ([]byte, error)` with retryablehttp + timeouts.
  - `ParseHistoryMarkdown(raw []byte) (DomainHistory, error)` – MVP: extract exercise, load, reps, RIR/RPE, date. (Heuristics ok.)
  - `ParseHistoryYAML/JSON` stubs for later.
- [ ] Unit tests with golden files (synthetic 90‑day sample).

## 6. Strava client (personal token)
- [x] `internal/strava`:
  - `Client{http *retryablehttp.Client, token string}`
  - `GetRecentActivities(ctx, sinceDays int) ([]Activity, error)` – includes Relative Effort.
- [x] Unit tests: round‑trip against recorded JSON fixtures (no network).

## 7. LLM client & prompts
- [ ] `internal/llm`:
  - `Analyze(ctx, inputs AnalyzerInputs) (AnalyzerPlan, error)`
  - `Generate(ctx, plan AnalyzerPlan) ([]byte /*YAML*/, error)`
  - Uses prompts from `/prompts` folder (read at runtime).
  - JSON mode for analyzer, plain text for generator.
  - Retries: up to `LLM_RETRIES` on validation failure with repair prompt.
- [ ] Unit tests: mock HTTP to LLM; validate schema pass/fail paths.

## 8. Workout domain & timeboxing
- [ ] `internal/workout`:
  - Data types for domain objects (tiers, sets).
  - Helpers to estimate set counts from `duration_minutes` and rests.
  - Apply fatigue policy (RIR shift, load caps).
- [ ] Unit tests: timeboxing math and fatigue policy behavior.

## 9. HTTP API (private)
- [ ] `cmd/swolegen-api/main.go` using Fiber:
  - `/healthz` (200 OK)
  - `/readyz` (schema load + env present)
  - `/v1/generate` (private; reads JSON payload with URLs + params; returns YAML)
- [ ] Unit tests: handler logic with in‑memory deps.

## 10. CI/CD
- [ ] GitHub Actions: `go vet`, unit tests, schema validation on examples, lint.
- [ ] Deploy to Dokku on merges to `main` (Dockerfile or buildpack as configured).

## 11. Docs & examples
- [ ] Add `examples/analyzer-v1.example.json` and `examples/workout-v1.2.example.yaml` that validate.
- [ ] Update README links to prompts and schemas.

---

## Nice‑to‑have (after MVP)
- OAuth for Strava
- CSV history import
- Multi‑day planning
- Metrics (latency per endpoint)
- e1RM‑based safety caps toggle
