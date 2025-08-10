# Architecture Decisions & Notes

## Go & Modules
- Target **Go 1.23**. Use modules. Release builds with `-trimpath`. No special module proxy behavior required.

## Process Model
- **Monolith** with `internal/` packages. Single binary: `cmd/swolegen-api`.

## HTTP & Routing
- Router: **Fiber** (your preference). Endpoints for MVP (private):
  - `GET /healthz` – liveness
  - `GET /readyz` – schemas loaded and required env present
  - `POST /v1/generate` – non-public; runs analyzer+generator and returns YAML

## Data & Schema
- Schemas in `/schemas` are the **source of truth**.
- Conversion question (YAML <-> JSON): We only need **YAML→JSON** to validate the generated workout against the JSON Schema. The analyzer remains JSON-only.

## IDs & Determinism
- Hash algorithm: **xxhash** for speed and stable 64-bit hashing. Use modulo 100 for the `NN` seed (`%02d` formatting).
- Exercise slugging: uppercase `A–Z0–9–` only, max 12 characters. Examples: `RDL`, `DBIP`, `PULLUP`.

## External Services
- **Strava**: MVP uses a **personal token** via env var. OAuth deferred.

## LLM Options (Analyzer/Generator)
You have two main API surfaces in OpenAI:

1. **Chat Completions API**
   - Pros: mature; widely used; straightforward for system/user prompts; supports JSON mode.
   - Cons: legacy in some areas; migration path to new APIs may be needed later.

2. **Responses API**
   - Pros: newer; unifies input/output; tool invocation patterns; better structured output controls.
   - Cons: docs and SDK behavior can evolve faster; examples less abundant.

**Recommendation for MVP**: Use **Chat Completions** with **JSON mode** for the Analyzer and plain-text for the Generator. It’s simple, stable, and easy to mock in tests.

### Cost & Token Controls
- Set **`max_tokens`** via env (`LLM_MAX_TOKENS_ANALYZER`, `LLM_MAX_TOKENS_GENERATOR`).
- Set **`temperature`** low for analyzer (`0–0.2`), moderate for generator (`0.3–0.5`).
- Implement a **circuit breaker**: if more than `N` failures in `T` minutes, stop calling LLM and return 503 with a retry-after.
- Per-request **timeout**: analyzer 90s, generator 60s (env-configurable).

## Validation & Retries
- Validate **both** Analyzer JSON and Workout YAML against schemas.
- On failure, send **repair prompt** and retry up to **3 times**. If still invalid, return structured error including validator messages.

## History Ingestion
- MVP assumes **public URLs** (e.g., Gist raw).
- Since history may be raw Markdown, add a step: **AI-assisted cleanup** inside `internal/history` that extracts {exercise, load, reps, RIR/RPE, date}. Use regex first; optional LLM fallback.

## Persistence & Config
- **Stateless** server. If temporary storage becomes necessary, use S3 (Spaces compatible) with short TTL.
- Env-only config. (FYI: **Viper** is a Go config library supporting env, flags, files; we can skip it and read `os.Getenv` directly for MVP.)

## Testing
- Use Go’s **built-in testing**. Golden files for example JSON/YAML fixtures in `/examples`.

## Observability & Ops
- Logging via `log/slog` with JSON handler; include request IDs.
- Skip metrics/OTel for MVP.

## Build & Deploy
- Dokku buildpack compatible. You can also provide a multi-stage Dockerfile.
- GitHub Actions deploy to Dokku on merge to `main`.
