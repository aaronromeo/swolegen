package llm

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/aaronromeo/swolegen/internal/llm/generated"
	"github.com/aaronromeo/swolegen/internal/llm/provider"
	"github.com/atombender/go-jsonschema/pkg/types"
)

type fakeProvider struct {
	reply string
	err   error
}

func (f fakeProvider) Complete(ctx context.Context, prf provider.ProviderResponseFormat) (string, error) {
	return f.reply, f.err
}

func (f fakeProvider) Validate() error { return nil }

func TestAnalyze_Success(t *testing.T) {
	minSetsPerSelectedPattern := 2
	plan := generated.AnalyzerV1Json{
		Meta: generated.AnalyzerV1JsonMeta{
			Date: types.SerializableDate{Time: time.Date(2023, 10, 01, 0, 0, 0, 0, time.UTC)}, Location: "gym", Units: "lbs", DurationMinutes: 45, Goal: "hypertrophy",
			SupersetPolicy:      "pairs_ok",
			SupersetPreferences: generated.AnalyzerV1JsonMetaSupersetPreferences{},
		},
		Session: generated.AnalyzerV1JsonSession{
			Type: "strength", Tiers: []generated.AnalyzerV1JsonSessionTiersElem{"A", "B"}, CutOrder: []generated.AnalyzerV1JsonSessionCutOrderElem{"B"},
		},
		GapFillPolicy: generated.AnalyzerV1JsonGapFillPolicy{MinSetsPerSelectedPattern: &minSetsPerSelectedPattern, TargetPatterns: []string{"push_horizontal"}},
		FatiguePolicy: generated.AnalyzerV1JsonFatiguePolicy{RirShift: 1, LoadCapPct: 0.9, Reason: ""},
		InstructionsContext: generated.AnalyzerV1JsonInstructionsContext{
			PrimaryGoals:        []string{"hypertrophy"},
			ExecutionPrinciples: []string{"controlled_tempo"},
			ConstructionRules: generated.AnalyzerV1JsonInstructionsContextConstructionRules{
				Format:                  "supersets",
				PriorityOrder:           []generated.AnalyzerV1JsonInstructionsContextConstructionRulesPriorityOrderElem{"big_compound"},
				RestBetweenSupersetsSec: nil,
			},
			Constraints: generated.AnalyzerV1JsonInstructionsContextConstraints{
				Avoid:               []string{},
				Encourage:           []string{},
				PreferSingleStation: nil,
			},
		},
		TimeBudget:         generated.AnalyzerV1JsonTimeBudget{EstimatedMinutesTotal: nil, TargetSetCount: 12},
		AvailableEquipment: []string{"barbell"},
		ExercisePlan: []generated.AnalyzerV1JsonExercisePlanElem{
			{
				Tier:        "A",
				Exercise:    "Bench Press",
				Equipment:   "barbell",
				Warmups:     1,
				WorkingSets: 3,
				Targets: func() generated.AnalyzerV1JsonExercisePlanElemTargets {
					r := 2
					return generated.AnalyzerV1JsonExercisePlanElemTargets{RepRange: "6-8", Rir: &r, TargetLoad: nil, LoadCap: nil}
				}(),
			},
		},
	}
	okJSON, err := json.Marshal(&plan)
	if err != nil {
		t.Fatalf("ToJSON: %v", err)
	}
	cli, err := New(
		WithProvider(fakeProvider{reply: string(okJSON)}),
		WithLogger(slog.Default()),
	)
	if err != nil {
		t.Fatalf("New Provider Error: %v", err)
	}

	in := AnalyzerInputs{InstructionsURL: "", HistoryURL: "", Location: "gym", EquipmentInventory: []string{"barbell"}, DurationMinutes: 45, Units: "lbs"}
	got, err := cli.Analyze(context.Background(), in)
	if err != nil {
		t.Fatalf("Analyze error: %v", err)
	}
	if got.Meta.Location != "gym" || len(got.ExercisePlan) == 0 {
		t.Fatalf("unexpected plan: %+v", got)
	}
}

func TestAnalyze_RepairAfterInvalid(t *testing.T) {
	minSetsPerSelectedPattern := 2
	bad := `{"not_valid": true}`
	bpRir := 2
	plan := generated.AnalyzerV1Json{
		Meta: generated.AnalyzerV1JsonMeta{
			Date: types.SerializableDate{Time: time.Date(2023, 10, 01, 0, 0, 0, 0, time.UTC)}, Location: "gym", Units: "lbs", DurationMinutes: 45, Goal: "hypertrophy",
			SupersetPolicy:      "pairs_ok",
			SupersetPreferences: generated.AnalyzerV1JsonMetaSupersetPreferences{},
		},
		Session: generated.AnalyzerV1JsonSession{
			Type: "strength", Tiers: []generated.AnalyzerV1JsonSessionTiersElem{"A", "B"}, CutOrder: []generated.AnalyzerV1JsonSessionCutOrderElem{"B"},
		},
		GapFillPolicy: generated.AnalyzerV1JsonGapFillPolicy{MinSetsPerSelectedPattern: &minSetsPerSelectedPattern, TargetPatterns: []string{"push_horizontal"}},

		FatiguePolicy: generated.AnalyzerV1JsonFatiguePolicy{
			RirShift: 1, LoadCapPct: 0.9, Reason: "",
		},
		InstructionsContext: generated.AnalyzerV1JsonInstructionsContext{
			PrimaryGoals:        []string{"hypertrophy"},
			ExecutionPrinciples: []string{"controlled_tempo"},
			ConstructionRules: generated.AnalyzerV1JsonInstructionsContextConstructionRules{
				Format:                  "supersets",
				PriorityOrder:           []generated.AnalyzerV1JsonInstructionsContextConstructionRulesPriorityOrderElem{"big_compound"},
				RestBetweenSupersetsSec: nil,
			},
			Constraints: generated.AnalyzerV1JsonInstructionsContextConstraints{
				Avoid:               []string{},
				Encourage:           []string{},
				PreferSingleStation: nil,
			},
		},
		TimeBudget:         generated.AnalyzerV1JsonTimeBudget{EstimatedMinutesTotal: nil, TargetSetCount: 12},
		AvailableEquipment: []string{"barbell"},
		ExercisePlan: []generated.AnalyzerV1JsonExercisePlanElem{
			{Tier: "A", Exercise: "Bench Press", Equipment: "barbell", Warmups: 1, WorkingSets: 3, Targets: generated.AnalyzerV1JsonExercisePlanElemTargets{RepRange: "6-8", Rir: &bpRir, TargetLoad: nil, LoadCap: nil}},
		},
	}
	okJSON, err := json.Marshal(&plan)
	if err != nil {
		t.Fatalf("ToJSON: %v", err)
	}
	p := &sequenceProvider{replies: []string{bad, string(okJSON)}}
	cli, err := New(
		WithProvider(p),
		WithRetries(2),
		WithLogger(slog.Default()),
	)
	if err != nil {
		t.Fatalf("New Provider Error: %v", err)
	}

	in := AnalyzerInputs{Location: "gym", EquipmentInventory: []string{"barbell"}, DurationMinutes: 45, Units: "lbs"}
	got, err := cli.Analyze(context.Background(), in)
	if err != nil {
		t.Fatalf("Analyze error after repair: %v", err)
	}
	if got.Meta.Location != "gym" {
		t.Fatalf("unexpected plan: %+v", got)
	}
}

type sequenceProvider struct {
	replies []string
	i       int
}

func (s *sequenceProvider) Complete(ctx context.Context, prf provider.ProviderResponseFormat) (string, error) {
	if s.i >= len(s.replies) {
		return "", errors.New("no more replies")
	}
	r := s.replies[s.i]
	s.i++
	return r, nil
}

func (s *sequenceProvider) Validate() error { return nil }

func TestFetchText_CapsSize(t *testing.T) {
	// Serve a large response and ensure we cap reads
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		if _, err := w.Write([]byte(strings.Repeat("x", 200000))); err != nil {
			t.Fatalf("write: %v", err)
		}
	}))
	defer ts.Close()
	got, err := fetchText(context.Background(), ts.URL, 1024)
	if err != nil {
		t.Fatalf("fetchText: %v", err)
	}
	if len(got) != 1024 {
		t.Fatalf("expected 1024 bytes, got %d", len(got))
	}
}

func TestFetchToTmp_CreatesAndOptionallyRemoves(t *testing.T) {
	// Use a file URL
	f, err := os.CreateTemp("", "swolegen-test-*.txt")
	if err != nil {
		t.Fatalf("tmp: %v", err)
	}
	defer func() {
		if err := os.Remove(f.Name()); err != nil {
			t.Fatalf("remove: %v", err)
		}
	}()
	if _, err := f.WriteString("hello world"); err != nil {
		t.Fatalf("write: %v", err)
	}
	if err := f.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
	p, content, err := fetchToTmp(context.Background(), "file://"+f.Name(), "instructions", 65536)
	if err != nil {
		t.Fatalf("fetchToTmp: %v", err)
	}
	if content != "hello world" {
		t.Fatalf("content mismatch: %q", content)
	}
	if _, statErr := os.Stat(p); statErr == nil {
		if _, err2 := os.Stat(p); err2 == nil {
			t.Fatalf("expected tmp file to be removed: %s", p)
		}
	}
}
