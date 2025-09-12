package llm

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/aaronromeo/swolegen/internal/llm/generated"
	"github.com/aaronromeo/swolegen/internal/llm/provider"
	"gopkg.in/yaml.v3"
)

const (
	defaultRetries       = 3
	defaultMaxFetchBytes = 65536
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

type HistoryInputs struct {
	// history_url – set-level history (load × reps × RIR/RPE, time/date, exercise name).
	HistoryURL string `json:"history_url"`
}

type Client struct {
	provider      provider.Provider
	retries       int
	maxFetchBytes int
	logger        *slog.Logger
}

type LLMClientOption func(*Client)

func WithProvider(p provider.Provider) LLMClientOption {
	return func(c *Client) {
		c.provider = p
	}
}

func WithLogger(logger *slog.Logger) LLMClientOption {
	return func(c *Client) {
		c.logger = logger
	}
}

func WithRetries(n int) LLMClientOption {
	return func(c *Client) {
		c.retries = n
	}
}

func WithMaxFetchBytes(n int) LLMClientOption {
	return func(c *Client) {
		c.maxFetchBytes = n
	}
}

func New(opts ...LLMClientOption) (*Client, error) {
	c := &Client{
		retries:       defaultRetries,
		maxFetchBytes: defaultMaxFetchBytes,
	}
	for _, opt := range opts {
		opt(c)
	}

	if err := c.provider.Validate(); err != nil {
		return nil, err
	}
	return c, nil
}

func (c *Client) Validate() error {
	if c.provider == nil {
		return errors.New("llm provider not configured")
	}
	if err := c.provider.Validate(); err != nil {
		return err
	}
	if c.retries < 0 {
		return errors.New("llm retries must be non-negative")
	}
	if c.maxFetchBytes <= 0 {
		return errors.New("llm max fetch bytes must be positive")
	}
	if c.logger == nil {
		return errors.New("llm logger not configured")
	}
	return nil
}

func (c *Client) IngestHistory(ctx context.Context, in HistoryInputs) (generated.HistoryV1Json, error) {
	if c.provider == nil {
		return generated.HistoryV1Json{}, errors.New("llm provider not configured")
	}
	if err := c.Validate(); err != nil {
		return generated.HistoryV1Json{}, err
	}

	historyText, err := fetchText(ctx, in.HistoryURL, c.maxFetchBytes)
	if err != nil {
		return generated.HistoryV1Json{}, fmt.Errorf("fetch history: %w", err)
	}
	c.logger.Debug("ingest history", "history", historyText)

	// indent multi-line blocks for YAML literal style
	historyBlock := indentForBlock(historyText)

	user := fmt.Sprintf(HistoryUser, time.Now().Format("2006-01-30"), historyBlock)

	userJSON, err := json.Marshal(user)
	if err != nil {
		return generated.HistoryV1Json{}, fmt.Errorf("marshal user prompt: %w", err)
	}

	errs := []error{}
	history := generated.HistoryV1Json{}

	// initial completion
	prf := provider.ProviderResponseFormat{
		Name:         provider.ResponseFormatHistoryPlan,
		Description:  provider.ResponseFormatHistoryPlanDescription,
		Schema:       HistorySchema,
		SystemPrompt: HistorySystem,
		UserPrompt:   string(userJSON),
	}

	for i := 0; i < c.retries; i++ {
		llmOut, err := c.provider.Complete(ctx, prf)

		if err != nil {
			c.logger.Error("ingest history", "error", err)
			errs = append(errs, err)
			goto retryCase
		}

		c.logger.Debug("ingest history", "llm_out", llmOut)

		if err = json.Unmarshal([]byte(llmOut), &history); err != nil {
			err = fmt.Errorf("failed to parse ingest history: %w", err)
			c.logger.Error("ingest history", "error", err)
			errs = append(errs, err)
			goto retryCase
		}

		c.logger.Debug("ingest history", "history", history)
		return history, nil

	retryCase:
		repairUser := fmt.Sprintf(RepairHistory, errors.Join(errs...).Error(), string(userJSON))
		prf = provider.ProviderResponseFormat{
			Name:         provider.ResponseFormatHistoryPlan,
			Description:  provider.ResponseFormatHistoryPlanDescription,
			Schema:       HistorySchema,
			SystemPrompt: HistorySystem,
			UserPrompt:   repairUser,
		}
	}
	return generated.HistoryV1Json{}, errors.Join(errs...)
}

// Analyze assembles prompts, calls the provider, and parses the plan.
func (c *Client) Analyze(ctx context.Context, in AnalyzerInputs) (generated.AnalyzerV1Json, error) {
	if c.provider == nil {
		return generated.AnalyzerV1Json{}, errors.New("llm provider not configured")
	}
	if err := c.Validate(); err != nil {
		return generated.AnalyzerV1Json{}, err
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
		return generated.AnalyzerV1Json{}, fmt.Errorf("marshal equipment_inventory: %w", err)
	}

	instructionsText, err := fetchText(ctx, in.InstructionsURL, c.maxFetchBytes)
	if err != nil {
		return generated.AnalyzerV1Json{}, fmt.Errorf("fetch instructions: %w", err)
	}
	c.logger.Debug("analyzer plan", "instructions", instructionsText)

	historyText, err := fetchText(ctx, in.HistoryURL, c.maxFetchBytes)
	if err != nil {
		return generated.AnalyzerV1Json{}, fmt.Errorf("fetch history: %w", err)
	}
	c.logger.Debug("analyzer plan", "history", historyText)

	// indent multi-line blocks for YAML literal style
	instructionsBlock := indentForBlock(instructionsText)
	historyBlock := indentForBlock(historyText)

	user := fmt.Sprintf(AnalyzerUser,
		instructionsBlock, historyBlock, stravaJSON, in.UpcomingCardioText,
		sleep, bb, string(invJSON), date, in.Location, units, in.DurationMinutes,
	)

	userJSON, err := json.Marshal(user)
	if err != nil {
		return generated.AnalyzerV1Json{}, fmt.Errorf("marshal user prompt: %w", err)
	}

	errs := []error{}
	plan := generated.AnalyzerV1Json{}

	// initial completion
	prf := provider.ProviderResponseFormat{
		Name:         provider.ResponseFormatAnalyzerPlan,
		Description:  provider.ResponseFormatAnalyzerPlanDescription,
		Schema:       AnalyzerSchema,
		SystemPrompt: AnalyzerSystem,
		UserPrompt:   string(userJSON),
	}

	for i := 0; i < c.retries; i++ {
		llmOut, err := c.provider.Complete(ctx, prf)

		if err != nil {
			c.logger.Error("analyzer plan", "error", err)
			errs = append(errs, err)
			goto retryCase
		}

		if err = json.Unmarshal([]byte(llmOut), &plan); err != nil {
			err = fmt.Errorf("failed to parse analyzer plan: %w", err)
			c.logger.Error("analyzer plan", "error", err)
			errs = append(errs, err)
			goto retryCase
		}

		c.logger.Debug("analyzer plan", "plan", plan)
		return plan, nil

	retryCase:
		repairUser := fmt.Sprintf(RepairAnalyzer, errors.Join(errs...).Error(), string(userJSON))
		prf = provider.ProviderResponseFormat{
			Name:         provider.ResponseFormatAnalyzerPlan,
			Description:  provider.ResponseFormatAnalyzerPlanDescription,
			Schema:       AnalyzerSchema,
			SystemPrompt: AnalyzerSystem,
			UserPrompt:   repairUser,
		}
	}
	return generated.AnalyzerV1Json{}, errors.Join(errs...)
}

func (c *Client) Generate(ctx context.Context, plan generated.AnalyzerV1Json) ([]byte, error) {
	if c.provider == nil {
		return nil, errors.New("llm provider not configured")
	}
	if err := c.Validate(); err != nil {
		return nil, err
	}

	// Convert plan to JSON for the prompt
	planJSON, err := json.Marshal(&plan)
	if err != nil {
		return nil, fmt.Errorf("marshal analyzer plan: %w", err)
	}

	// Build user prompt with the plan
	userJSON := fmt.Sprintf(GeneratorUser, string(planJSON))

	errs := []error{}
	workout := &generated.WorkoutV12Json{}

	prf := provider.ProviderResponseFormat{
		Name:         provider.ResponseFormatGeneratorOutput,
		Description:  provider.ResponseFormatGeneratorOutputDescription,
		Schema:       WorkoutSchema,
		SystemPrompt: GeneratorSystem,
		UserPrompt:   userJSON,
	}
	for i := 0; i < c.retries; i++ {
		llmOut, err := c.provider.Complete(ctx, prf)

		if err != nil {
			c.logger.Error("workout json", "error", err)
			errs = append(errs, err)
			goto retryCase
		}

		if err = json.Unmarshal([]byte(llmOut), workout); err != nil {
			err = fmt.Errorf("failed to parse workout json: %w", err)
			c.logger.Error("workout json", "error", err)
			errs = append(errs, err)
			goto retryCase
		}

		// // Semantic validation beyond schema
		// if err = ValidateWorkoutSemantics(&plan, workout); err != nil {
		// 	err = fmt.Errorf("workout semantic validation: %w", err)
		// 	c.logger.Error("workout semantics", "error", err)
		// 	errs = append(errs, err)
		// 	goto retryCase
		// }

		return yaml.Marshal(workout)

	retryCase:
		repairUser := fmt.Sprintf(RepairGenerator, errors.Join(errs...).Error(), string(userJSON))
		prf = provider.ProviderResponseFormat{
			Name:         provider.ResponseFormatGeneratorOutput,
			Description:  provider.ResponseFormatGeneratorOutputDescription,
			Schema:       WorkoutSchema,
			SystemPrompt: GeneratorSystem,
			UserPrompt:   repairUser,
		}
	}
	return nil, errors.Join(errs...)
}
