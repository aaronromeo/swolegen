package llm

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

type fakeProvider struct {
	reply string
	err   error
}

func (f fakeProvider) Complete(ctx context.Context, prf ProviderResponseFormat) (string, error) {
	return f.reply, f.err
}

func TestAnalyze_Success(t *testing.T) {
	plan := AnalyzerPlan{
		Meta:          AnalyzerMeta{Date: "2023-10-01", Location: "gym", Units: "lbs", DurationMinutes: 45, Goal: "hypertrophy"},
		Session:       AnalyzerSession{Type: "strength", Tiers: []string{"A"}, CutOrder: []string{"A"}},
		FatiguePolicy: AnalyzerFatiguePolicy{RIRShift: 1, LoadCapPct: 0.95, Reason: "ok"},
		TimeBudget:    AnalyzerTimeBudget{SetSecondsEstimate: 120, TargetSetCount: 20},
		ExercisePlan:  []ExercisePlanItem{{Tier: "A", Exercise: "Bench Press", Equipment: "barbell", Warmups: 2, WorkingSets: 4, Targets: ExerciseTargets{RepRange: "6-8", RIR: 2}}},
	}
	okJSON, err := plan.ToJSON()
	if err != nil {
		t.Fatalf("ToJSON: %v", err)
	}
	cli := NewWithProvider(fakeProvider{reply: string(okJSON)})

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
	bad := `{"not_valid": true}`
	plan := AnalyzerPlan{Meta: AnalyzerMeta{Date: "2023-10-01", Location: "gym", Units: "lbs", DurationMinutes: 45, Goal: "hypertrophy"}, Session: AnalyzerSession{Type: "strength", Tiers: []string{"A"}, CutOrder: []string{"A"}}, FatiguePolicy: AnalyzerFatiguePolicy{RIRShift: 1, LoadCapPct: 0.9, Reason: ""}, TimeBudget: AnalyzerTimeBudget{SetSecondsEstimate: 100, TargetSetCount: 12}, ExercisePlan: []ExercisePlanItem{{Tier: "A", Exercise: "Bench Press", Equipment: "barbell", Warmups: 1, WorkingSets: 3, Targets: ExerciseTargets{RepRange: "6-8", RIR: 2}}}}
	okJSON, err := plan.ToJSON()
	if err != nil {
		t.Fatalf("ToJSON: %v", err)
	}
	p := &sequenceProvider{replies: []string{bad, string(okJSON)}}
	cli := NewWithProvider(p)
	if err := os.Setenv("LLM_RETRIES", "2"); err != nil {
		t.Fatalf("Setenv: %v", err)
	}
	t.Cleanup(func() {
		if err := os.Unsetenv("LLM_RETRIES"); err != nil {
			t.Fatalf("Unsetenv: %v", err)
		}
	})
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

func (s *sequenceProvider) Complete(ctx context.Context, prf ProviderResponseFormat) (string, error) {
	if s.i >= len(s.replies) {
		return "", errors.New("no more replies")
	}
	r := s.replies[s.i]
	s.i++
	return r, nil
}

func TestFetchText_CapsSize(t *testing.T) {
	// Serve a large response and ensure we cap reads
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		if _, err := w.Write([]byte(strings.Repeat("x", 200000))); err != nil {
			t.Fatalf("write: %v", err)
		}
	}))
	defer ts.Close()
	if err := os.Setenv("LLM_MAX_FETCH_BYTES", "1024"); err != nil {
		t.Fatalf("Setenv: %v", err)
	}
	defer func() {
		if err := os.Unsetenv("LLM_MAX_FETCH_BYTES"); err != nil {
			t.Fatalf("Unsetenv: %v", err)
		}
	}()
	got, err := fetchText(context.Background(), ts.URL)
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
	p, content, err := fetchToTmp(context.Background(), "file://"+f.Name(), "instructions")
	if err != nil {
		t.Fatalf("fetchToTmp: %v", err)
	}
	if content != "hello world" {
		t.Fatalf("content mismatch: %q", content)
	}
	if _, statErr := os.Stat(p); statErr == nil {
		// should be removed unless LLM_DEBUG set
		if os.Getenv("LLM_DEBUG") == "" {
			if _, err2 := os.Stat(p); err2 == nil {
				t.Fatalf("expected tmp file to be removed: %s", p)
			}
		}
	}
}
