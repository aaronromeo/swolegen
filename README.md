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

A full JSON Schema for validation is defined in [`schemas/validator-workout-v1.json`](schemas/validator-workout-v1.json).

---

## Processing Flow

1. **Analyzer (Step 1)**
   - Input: all user data (instructions, history, Strava, recovery, equipment, etc.).
   - Output: compact JSON plan ([`schemas/validator-analyzer-v1.json`](schemas/validator-analyzer-v1.json)) selecting session type, tiers, fatigue policy, time budget, and per-exercise set counts/targets.
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

- **Analyzer JSON Schema**: [`schemas/validator-analyzer-v1.json`](schemas/validator-analyzer-v1.json)
- **Workout YAML Schema**: [`schemas/validator-workout-v1.json`](schemas/validator-workout-v1.json)
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

## References

- [Output YAML Gist](docs/sample-output.yaml)
- [JSON Schema: Workout](schemas/validator-workout-v1.json)
- [JSON Schema: Analyzer](schemas/validator-analyzer-v1.json)
