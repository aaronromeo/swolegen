# SwoleGen LLM Request Flow (Analyzer & Generator)

This document summarizes **how to call the LLM twice**—first to analyze inputs into a compact plan, then to generate the final YAML workout—explicitly clarifying **what goes into the `system` vs `user` messages** for each step.

---

## Overview

- **Step 1 — Analyzer (JSON output)**  
  Purpose: Normalize all inputs (instructions, history, Strava/recovery, equipment, time) into a **structured plan** that the generator can consume. Output must validate against `schemas/analyzer-v1.json`.

- **Step 2 — Generator (YAML output)**  
  Purpose: Convert the Analyzer plan into a **pre-enumerated workout YAML** (one node per set, user-friendly logging fields). Output must validate against `schemas/workout-v1.2.json`.

Each step uses a different **system vs user** prompt split:
- **System**: policy, invariants, schema contract, formatting constraints.
- **User**: concrete, request-specific inputs and data (resolved text/JSON) and the schema blob (for strong steering).

---

## Models, Modes, and Guards

- **Models (MVP recommendation)**:  
  - Analyzer: `gpt-4o-mini`, **JSON mode** (low temperature, e.g. 0.1).  
  - Generator: `gpt-4o-mini`, **plain text** (YAML-only), temperature 0.3–0.5.
- **Timeouts** (env): Analyzer 90s, Generator 60s.
- **Retries/repairs**: On schema validation failure, **retry up to 3 times** with a short repair prompt (include validator error lines).

---

## Step 1 — Analyzer Call (to JSON)

### Purpose
Turn messy multi-source inputs into a deterministic **session plan**: session type, tiers, per-exercise set counts/targets, fatigue policy, and timeboxing. The analyzer guards correctness (available equipment, anti-repeat) and sets constraints for the generator.

### Messages

**System (policy & invariants)** — *What the model must always follow*
- State the role: “You are the SwoleGen ANALYZER.”
- Rules:
  - Use last **90d** for recent bests and **14d** for anti-repeat (unless last strength day ≥ 7d ago).
  - Respect bans/injuries/preferences from user instructions.
  - Use **available equipment only**.
  - Apply **fatigue gating** (Strava load + optional recovery scores).
  - **Double progression** model; set conservative caps if history is thin.
  - **Output JSON only**, conforming to **Analyzer v1 JSON Schema**.
- Include **succinct progression & safety notes** (RIR bands, load caps, warmup policy).

**User (data payload & schema)** — *Request-specific content*
- Resolved inputs (examples):
  - `instructions_text` (inline markdown pasted from URL)
  - `history_json_or_markdown` (raw) — analyzer is allowed to ignore irrelevant lines
  - `strava_recent` (summarized list with relative effort)
  - `upcoming_cardio_text` (short free text)
  - `recovery_signals` (sleep score, body battery) – optional
  - `equipment_inventory` (checkbox keys you finalized)
  - `meta` (date, location, units, duration_minutes, goal)
- Embed **Analyzer v1 JSON Schema** at the end for the model to align with.

### Example (abbreviated)

- **system**: `prompts/analyzer-system.md` (as in repo)  
- **user**:
  ```
  Inputs:
  - instructions_text: <<markdown from URL>>
  - history_json_or_markdown: <<raw 90d block or JSON>>
  - strava_recent: <<last 7–14d with Relative Effort>>
  - upcoming_cardio_text: <<next 2–4d free text>>
  - recovery_signals: {"sleep_score": 72, "body_battery": 58}
  - equipment_inventory: ["eq_dumbbells_set_5-50","eq_cable_machine_dual_stack","eq_pullup_bar", ...]
  - meta: {"date":"2025-08-17","location":"gym:downtown","units":"lbs","duration_minutes":60,"goal":"hypertrophy"}

  Schema (for validation):
  <<contents of schemas/analyzer-v1.json>>
  ```

### Post-call Validation
- Parse the model output as JSON; validate with `schemas/analyzer-v1.json`.
- If invalid → build a **repair user prompt** with a concise bullet list of validator errors; retry (max 3).

### Output Contract
- A **compact JSON plan** with fields like:
  - `session_type`, `tiers` (A/B/C, plus warm-up policy), `time_budget`
  - `fatigue_policy` deltas (RIR shift, load cap factor)
  - `exercises[]` with `slug`, `equipment`, `target_sets`, `rep_range`, `rest_s`, `notes`
  - Any IDs or seeds needed for deterministic `workout_id` generation

---

## Step 2 — Generator Call (to YAML)

### Purpose
Render the Analyzer plan into a **ready-to-log YAML**: every set is explicitly listed with `target_weight`, `target_reps` (or range), `rir`, `rest_s`, and blank `actual_weight`/`actual_reps`. Warm-ups for first Tier A compound included. Output is **YAML-only** and must validate against `schemas/workout-v1.2.json`.

### Messages

**System (policy & format)** — *Immutable generation rules*
- State the role: “You are the SwoleGen GENERATOR.”
- Rules:
  - Output **YAML only**, strictly matching **Workout v1.2** schema.
  - **Enumerate one node per set** (logging-friendly).
  - **IDs**: `workout_id` = `YYYY-MM-DD-<kebab-location>-NN`; `set_id` = `<TIER>-<SLUG>-(WU#|#)`.
  - Tiers: `W` warm-up then `A/B/C`; `must: true` for `W` and `A`, false for `B/C`.
  - Apply analyzer’s **fatigue_policy** (RIR shift, load caps).
  - Honor equipment and bans; timebox by pushing overflow into `B`→`C` and set `cut_order`.
  - **Units: "lbs"** for MVP.

**User (plan + schema)** — *Request-specific content*
- The **Analyzer JSON** (already validated) pasted as-is.
- The **Workout v1.2 JSON Schema** blob to reinforce structure.

### Example (abbreviated)

- **system**: `prompts/generator-system.md`  
- **user**:
  ```
  Analyzer JSON:
  <<validated analyzer JSON from Step 1>>

  Workout Schema (for validation):
  <<contents of schemas/workout-v1.2.json>>
  ```

### Post-call Validation
- Validate the returned YAML by converting YAML→JSON and checking against `schemas/workout-v1.2.json`.
- If invalid → call **repair prompt** (user message) with validation errors; retry up to 3 times.

### Output Contract
- **YAML workout** with:
  - `version`, `workout_id`, `date`, `location`, `units`, `duration_minutes`, `goal`
  - `cut_order`, `notes_to_user` (optional)
  - `sets[]`: each set fully specified (`id`, `tier`, `must`, `superset?`, `order`, `exercise`, `equipment`, `target_reps` (int or "low-high"), `target_weight`, `rir`, `rest_s`, `actual_weight`, `actual_reps`, `notes`)
  - `post_workout` fields for user feedback

---

## Error Handling & Observability

- **Structured errors**: include validator messages and the stage (`analyzer` or `generator`) in responses.
- **Circuit breaker**: if repeated failures exceed threshold in a time window, return 503 with `Retry-After`.
- **Logging**: log request IDs, model, token usage, and validation outcome (no PII).

---

## Minimal Pseudocode

```
analyzerOut = callLLM(
  system = analyzerSystemText,
  user   = composeAnalyzerUser(inputs, analyzerSchema),
  model  = env.MODEL_ANALYZER,
  mode   = JSON
)
validate(analyzerOut, analyzerSchema) || retryRepair(...)

yamlOut = callLLM(
  system = generatorSystemText,
  user   = composeGeneratorUser(analyzerOut, workoutSchema),
  model  = env.MODEL_GENERATOR,
  mode   = TEXT
)
validateYAML(yamlOut, workoutSchema) || retryRepair(...)

return yamlOut
```

---

## Notes on System vs User

- **System** = guardrails + invariants + tone + formatting contract. Treat this as policy/config that changes rarely.
- **User** = volatile, per-request data (history, equipment, dates, goals) **and** the schema blob, which gives the model concrete structure to target for that one call.
- Keeping schemas in **user** improves steering without making the system prompt huge or version-coupled.
