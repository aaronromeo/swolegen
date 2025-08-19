package llm

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

// AnalyzerInputs is the input payload for the Analyzer prompt/LLM.
// JSON tags follow the external schema naming (snake_case) to keep
// serialization stable between services and docs.
type AnalyzerInputs struct {
	// instructions_url – user’s long-form rules (goals, bans, preferences).
	InstructionsURL string `json:"instructions_url"`
	// history_url – set-level history (load × reps × RIR/RPE, time/date, exercise name).
	HistoryURL string `json:"history_url"`
	// strava_recent – last 7–14 days activities with Relative Effort (or equivalent load).
	// Use RawMessage so callers can pass a pre-shaped JSON array/object without coupling here.
	StravaRecent json.RawMessage `json:"strava_recent,omitempty"`
	// upcoming_cardio_text – short free-text list of planned cardio next 2–4 days.
	UpcomingCardioText string `json:"upcoming_cardio_text,omitempty"`
	// location – string key: gym:<name> / home / hotel:<name>.
	Location string `json:"location"`
	// equipment_inventory – list of human names (e.g., "barbell", "db_set_5–100").
	EquipmentInventory []string `json:"equipment_inventory"`
	// duration_minutes – integer workout duration (e.g., 30, 45, 60).
	DurationMinutes int `json:"duration_minutes"`
	// units – "lbs" or "kg" (default "lbs").
	Units string `json:"units,omitempty"`
	// Optional recovery signals (0–100). Use pointers so omission is distinguishable from 0.
	GarminSleepScore  *int `json:"garmin_sleep_score,omitempty"`
	GarminBodyBattery *int `json:"garmin_body_battery,omitempty"`
}

// AnalyzerPlan mirrors schemas/analyzer-v1.json.
type AnalyzerPlan struct {
	Meta          AnalyzerMeta          `json:"meta"`
	Session       AnalyzerSession       `json:"session"`
	FatiguePolicy AnalyzerFatiguePolicy `json:"fatigue_policy"`
	TimeBudget    AnalyzerTimeBudget    `json:"time_budget"`
	ExercisePlan  []ExercisePlanItem    `json:"exercise_plan"`
}

type AnalyzerMeta struct {
	Date            string `json:"date"`
	Location        string `json:"location"`
	Units           string `json:"units"`
	DurationMinutes int    `json:"duration_minutes"`
	Goal            string `json:"goal"`
}

type AnalyzerSession struct {
	Type     string   `json:"type"`
	Tiers    []string `json:"tiers"`
	CutOrder []string `json:"cut_order"`
}

type AnalyzerFatiguePolicy struct {
	RIRShift   int     `json:"rir_shift"`
	LoadCapPct float64 `json:"load_cap_pct"`
	Reason     string  `json:"reason"`
}

type AnalyzerTimeBudget struct {
	SetSecondsEstimate int `json:"set_seconds_estimate"`
	TargetSetCount     int `json:"target_set_count"`
}

type ExerciseTargets struct {
	RepRange   string   `json:"rep_range"`
	RIR        int      `json:"rir"`
	TargetLoad *float64 `json:"target_load"`
	LoadCap    *float64 `json:"load_cap"`
}

type ExercisePlanItem struct {
	Tier        string          `json:"tier"`
	Exercise    string          `json:"exercise"`
	Equipment   string          `json:"equipment"`
	Superset    *string         `json:"superset"`
	Warmups     int             `json:"warmups"`
	WorkingSets int             `json:"working_sets"`
	Targets     ExerciseTargets `json:"targets"`
}

// ToJSON marshals the plan to JSON bytes.
func (p AnalyzerPlan) ToJSON() ([]byte, error) { return json.Marshal(p) }

// AnalyzerPlanFromJSON unmarshals bytes into a plan instance and validates it
// against the Analyzer v1 JSON Schema.
func AnalyzerPlanFromJSON(b []byte) (AnalyzerPlan, error) {
	if err := ValidateAnalyzerJSON(b); err != nil {
		return AnalyzerPlan{}, err
	}
	var p AnalyzerPlan
	return p, json.Unmarshal(b, &p)
}

type Client struct {
	p Provider
}

// CompletionTrace captures the prompts sent to the LLM and the raw response.
type CompletionTrace struct {
	Phase  string `json:"phase"`
	System string `json:"system"`
	User   string `json:"user"`
	Raw    string `json:"raw"`
}

func New() *Client                       { return &Client{} } // deprecated: prefer NewWithProvider or NewDefault
func NewWithProvider(p Provider) *Client { return &Client{p: p} }
func NewDefault() (*Client, error) {
	p, err := NewOpenAIProviderFromEnv()
	if err != nil {
		return nil, err
	}
	return &Client{p: p}, nil
}

// Analyze assembles prompts, calls the provider, and parses the plan.
func (c *Client) Analyze(ctx context.Context, in AnalyzerInputs) (AnalyzerPlan, error) {
	plan, _, err := c.AnalyzeWithDebug(ctx, in)
	return plan, err
}

// AnalyzeWithDebug returns the AnalyzerPlan and a trace of requests/responses for debugging.
func (c *Client) AnalyzeWithDebug(ctx context.Context, in AnalyzerInputs) (AnalyzerPlan, []CompletionTrace, error) {
	if c.p == nil {
		return AnalyzerPlan{}, nil, errors.New("llm provider not configured")
	}
	units := in.Units
	if strings.TrimSpace(units) == "" {
		units = "lbs"
	}
	date := time.Now().Format("2006-01-02")
	var stravaJSON string
	if len(in.StravaRecent) > 0 {
		stravaJSON = string(in.StravaRecent)
	} else {
		stravaJSON = "null"
	}
	sleep := "null"
	if in.GarminSleepScore != nil {
		sleep = fmt.Sprintf("%d", *in.GarminSleepScore)
	}
	bb := "null"
	if in.GarminBodyBattery != nil {
		bb = fmt.Sprintf("%d", *in.GarminBodyBattery)
	}
	invJSON, err := json.Marshal(in.EquipmentInventory)
	if err != nil {
		return AnalyzerPlan{}, nil, fmt.Errorf("marshal equipment_inventory: %w", err)
	}

	instructionsText, err := fetchText(ctx, in.InstructionsURL)
	if err != nil {
		return AnalyzerPlan{}, nil, fmt.Errorf("fetch instructions: %w", err)
	}
	historyText, err := fetchText(ctx, in.HistoryURL)
	if err != nil {
		return AnalyzerPlan{}, nil, fmt.Errorf("fetch history: %w", err)
	}
	// indent multi-line blocks for YAML literal style
	instructionsBlock := indentForBlock(instructionsText)
	historyBlock := indentForBlock(historyText)

	user := fmt.Sprintf(AnalyzerUser,
		instructionsBlock, historyBlock, stravaJSON, in.UpcomingCardioText,
		sleep, bb, string(invJSON), date, in.Location, units, in.DurationMinutes,
	)

	userJSON, err := json.Marshal(user)
	if err != nil {
		return AnalyzerPlan{}, nil, fmt.Errorf("marshal user prompt: %w", err)
	}

	var traces []CompletionTrace
	// initial completion
	out, err := c.p.Complete(ctx, AnalyzerSystem, string(userJSON))
	traces = append(traces, CompletionTrace{Phase: "initial", System: AnalyzerSystem, User: string(userJSON), Raw: out})
	if err != nil {
		return AnalyzerPlan{}, traces, err
	}
	plan, err := AnalyzerPlanFromJSON([]byte(out))
	if err == nil {
		return plan, traces, nil
	}

	// Retry loop using repair prompt if validation/parsing fails
	retries := 0
	if v := os.Getenv("LLM_RETRIES"); v != "" {
		if n, convErr := strconv.Atoi(v); convErr == nil && n >= 0 {
			retries = n
		}
	}
	lastErr := fmt.Errorf("failed to parse analyzer plan: %w", err)
	for i := 0; i < retries; i++ {
		repairUser := fmt.Sprintf(RepairAnalyzer, lastErr.Error(), AnalyzerSchema)
		out2, err2 := c.p.Complete(ctx, AnalyzerSystem, repairUser)
		traces = append(traces, CompletionTrace{Phase: fmt.Sprintf("repair-%d", i+1), System: AnalyzerSystem, User: repairUser, Raw: out2})
		if err2 != nil {
			lastErr = err2
			continue
		}

		if plan2, errParse := AnalyzerPlanFromJSON([]byte(out2)); errParse == nil {
			return plan2, traces, nil
		}
		lastErr = fmt.Errorf("failed to parse analyzer plan: %w", err)
	}
	return AnalyzerPlan{}, traces, lastErr
}

// fetchText downloads the content at a URL and returns it as a string.
// It supports http(s) and file URLs; for empty or invalid URLs, returns empty string.

// fetchToTmp downloads text and writes it to a temp file. Returns path and content.
func fetchToTmp(ctx context.Context, url, prefix string) (string, string, error) {
	content, err := fetchText(ctx, url)
	if err != nil {
		return "", "", err
	}
	f, err := os.CreateTemp("", "swolegen-"+prefix+"-*.txt")
	if err != nil {
		return "", content, err
	}
	defer func() {
		closeQuiet(f)
		if os.Getenv("LLM_DEBUG") == "" {
			removeQuiet(f.Name())
		}
	}()
	if _, err := f.WriteString(content); err != nil {
		return f.Name(), content, err
	}
	return f.Name(), content, nil
}

func fetchText(ctx context.Context, url string) (string, error) {
	url = strings.TrimSpace(url)
	if url == "" {
		return "", nil
	}
	capBytes := maxFetchBytes()
	// Local files support (file:// or relative path)
	if strings.HasPrefix(url, "file://") {
		p := strings.TrimPrefix(url, "file://")
		if strings.HasPrefix(url, "file://") {
			if pp, ok := strings.CutPrefix(url, "file://"); ok {
				p = pp
			}
		}
		f, err := os.Open(p)
		if err != nil {
			return "", err
		}
		defer closeQuiet(f)
		lr := &io.LimitedReader{R: f, N: int64(capBytes)}
		b, err := io.ReadAll(lr)
		if err != nil {
			return "", err
		}
		return string(b), nil
	}
	if !strings.HasPrefix(url, "http://") && !strings.HasPrefix(url, "https://") {
		// treat as local path
		f, err := os.Open(url)
		if err != nil {
			return "", err
		}
		defer closeQuiet(f)
		lr := &io.LimitedReader{R: f, N: int64(capBytes)}
		b, err := io.ReadAll(lr)
		if err != nil {
			return "", err
		}
		return string(b), nil
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer closeQuiet(resp.Body)
	if resp.StatusCode >= 300 {
		return "", fmt.Errorf("GET %s: %d", url, resp.StatusCode)
	}
	lr := &io.LimitedReader{R: resp.Body, N: int64(capBytes)}
	b, err := io.ReadAll(lr)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func maxFetchBytes() int {
	// Default to 64KB; configurable via LLM_MAX_FETCH_BYTES
	const def = 64 * 1024
	if v := os.Getenv("LLM_MAX_FETCH_BYTES"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			return n
		}
	}
	return def
}

// closeQuiet closes a Closer and swallows any error (for best-effort cleanup in defers).
func closeQuiet(c io.Closer) {
	if c == nil {
		return
	}
	if err := c.Close(); err != nil {
		// intentionally ignore; best-effort cleanup
	}
}

// removeQuiet removes a file path and swallows any error (for best-effort cleanup in defers).
func removeQuiet(name string) {
	if name == "" {
		return
	}
	if err := os.Remove(name); err != nil {
		// intentionally ignore; best-effort cleanup
	}
}

// indentForBlock indents each line by two spaces for YAML literal blocks.
func indentForBlock(s string) string {
	if s == "" {
		return ""
	}
	lines := strings.Split(s, "\n")
	for i := range lines {
		lines[i] = "  " + lines[i]
	}
	return strings.Join(lines, "\n")
}

func (c *Client) Generate(ctx context.Context, plan AnalyzerPlan) ([]byte, error) {
	return nil, errors.New("not implemented")
}
