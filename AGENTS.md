# Repository Guidelines

## Project Structure & Module Organization
- `cmd/` — entrypoints: `swolegen-api` (HTTP server) and `generators` (syncs equipment/fundamentals and updates schemas/UI assets).
- `internal/` — app code: `httpapi` (routes/OAuth/UI), `llm` (client, prompts, schemas, generated models), `config` (env parsing).
- `templates/` — prompt/schema sources (JSON/YAML) used by generators.
- `web/` — minimal demo UI served from `/`.
- `docs/`, `examples/`, `build/`, `scripts/` — reference, samples, build artifacts, helpers.

## Build, Test, and Development Commands
- `make build` — builds `./build/swolegen-api` from `cmd/swolegen-api`.
- `make test` — runs `go test ./...` with coverage to `cover.out`.
- `make lint` — `gofmt` check, `go vet`, then `golangci-lint` (errcheck enabled).
- `go run ./cmd/swolegen-api` — run server (reads `.env` via `internal/config`).
- `make normalize-generate` — refresh bundled schemas and generated Go models; also updates `web/equipment.yaml` and `templates/set-v1.json` enums.

## Coding Style & Naming Conventions
- Go 1.23; format with `gofmt` (tabs, std imports). Run `make lint` before PRs.
- Package names lowercase; exported types/functions use `CamelCase`; unexported use `lowerCamel`.
- Prefer context-aware functions and dependency injection (pass `*slog.Logger`, `*config.Config`).
- Errors: wrap with context; return `error` (no panics in handlers).

## Testing Guidelines
- Framework: standard `testing`. Place tests as `*_test.go`; name `TestXxx`.
- Use table-driven tests where practical; aim to cover core paths of `internal/httpapi` and `internal/llm`.
- Run `make test`; open coverage with `go tool cover -func=cover.out` as needed.

## Commit & Pull Request Guidelines
- Commit style: prefer Conventional Commits (`feat:`, `fix:`, `refactor:`, `docs:`, `test:`). Keep subject ≤72 chars; explain the “why” in the body.
- For changes affecting schemas or templates, note “schema:” in the scope and run `make normalize-generate` in the PR.
- PRs must include: summary, linked issues (e.g., `Fixes #123`), test plan (commands and results), and screenshots/GIFs for UI changes (`web/`).

## Security & Configuration Tips
- Do not commit secrets. `.env` is gitignored; set `OPENAI_API_KEY`, Strava vars (`STRAVA_CLIENT_ID`, `STRAVA_CLIENT_SECRET`, `STRAVA_REDIRECT_BASE_URL`, `STRAVA_SCOPES`, `STRAVA_STATE_SECRET`).
- When running locally: `set -a; source .env; set +a; ./build/swolegen-api`.
- Never persist access tokens server-side; the demo UI supplies Bearer tokens to `/strava/recent`.

