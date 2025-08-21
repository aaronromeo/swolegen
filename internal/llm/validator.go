package llm

import (
	"encoding/json"
	"fmt"

	"github.com/aaronromeo/swolegen/internal/llm/schemas"
)

func ValidateAnalyzerJSON(b []byte) (*schemas.AnalyzerV1Json, error) {
	avj := schemas.AnalyzerV1Json{}
	if err := json.Unmarshal(b, &avj); err != nil {
		return nil, fmt.Errorf("json parse: %w", err)
	}
	return &avj, nil
}

func ValidateWorkoutJSON(b []byte) (*schemas.WorkoutV12Json, error) {
	wv := schemas.WorkoutV12Json{}
	if err := json.Unmarshal(b, &wv); err != nil {
		return nil, fmt.Errorf("json parse: %w", err)
	}
	return &wv, nil
}
