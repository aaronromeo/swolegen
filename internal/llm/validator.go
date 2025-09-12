package llm

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/aaronromeo/swolegen/internal/llm/generated"
)

// JSON-level validation: ensure payloads can be parsed into generated structs.
func ValidateAnalyzerJSON(b []byte) (*generated.AnalyzerV1Json, error) {
	avj := generated.AnalyzerV1Json{}
	if err := json.Unmarshal(b, &avj); err != nil {
		return nil, fmt.Errorf("json parse: %w", err)
	}
	return &avj, nil
}

func ValidateWorkoutJSON(b []byte) (*generated.WorkoutV12Json, error) {
	wv := generated.WorkoutV12Json{}
	if err := json.Unmarshal(b, &wv); err != nil {
		return nil, fmt.Errorf("json parse: %w", err)
	}
	return &wv, nil
}

// Semantic validation beyond JSON Schema
// Rules can inspect both the analyzer plan and the generated workout.

type Rule interface {
	Validate(plan *generated.AnalyzerV1Json, workout *generated.WorkoutV12Json) error
	Name() string
}

// SupersetPolicyRule enforces a relationship between plan.meta.superset_policy and
// the presence/absence of supersets in the generated workout sets.
//
// Behavior:
// - "required": workout must include at least one set with a non-nil superset tag
// - "none": workout must include no supersets
// - other values (e.g., "pairs_ok", "giant_sets_ok", "auto_when_time_limited"): no hard enforcement
//
// Note: "required" is not currently one of the enum values, but we support it to
// allow forward-compatible enforcement without having to regenerate types.
type SupersetPolicyRule struct{}

func (SupersetPolicyRule) Name() string { return "superset_policy" }

func (SupersetPolicyRule) Validate(plan *generated.AnalyzerV1Json, workout *generated.WorkoutV12Json) error {
	policy := string(plan.Meta.SupersetPolicy)
	if policy == "" {
		// defaulting handled in generated Unmarshal; treat empty as no strict enforcement
		return nil
	}
	// Compute whether any set has a superset tag
	hasSupersets := false
	for _, s := range workout.Sets {
		if s.Superset != nil && *s.Superset != "" {
			hasSupersets = true
			break
		}
	}
	switch policy {
	case "required":
		if !hasSupersets {
			return errors.New("superset_policy=required but workout contains no supersets")
		}
	case "none":
		if hasSupersets {
			return errors.New("superset_policy=none but workout contains supersets")
		}
	default:
		// pairs_ok, giant_sets_ok, auto_when_time_limited => no hard requirement
	}
	return nil
}

// ValidateWorkoutSemantics applies all business rules that go beyond JSON schema.
// Returns nil if all rules pass.
func ValidateWorkoutSemantics(plan *generated.AnalyzerV1Json, workout *generated.WorkoutV12Json) error {
	rules := []Rule{
		SupersetPolicyRule{},
	}
	var errs []error
	for _, r := range rules {
		if err := r.Validate(plan, workout); err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", r.Name(), err))
		}
	}
	if len(errs) > 0 {
		return errors.Join(errs...)
	}
	return nil
}
