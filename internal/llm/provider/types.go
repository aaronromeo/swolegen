package provider

import (
	"context"
	"log/slog"

	"github.com/openai/openai-go/v2"
)

const (
	DefaultModel                             = "gpt-4.1-mini"
	ResponseFormatAnalyzerPlan               = "analyzer_plan"
	ResponseFormatAnalyzerPlanDescription    = "Workout analyzer plan JSON"
	ResponseFormatGeneratorOutput            = "generator_output"
	ResponseFormatGeneratorOutputDescription = "Workout generator output YAML"
)

// Provider defines the minimal interface for LLM completion.
type Provider interface {
	Complete(ctx context.Context, req ProviderResponseFormat) (string, error)
	Validate() error
}

// OpenAIProvider implements Provider using the official openai-go client.
type OpenAIProvider struct {
	apiKey string
	model  string

	Client openai.Client
	logger *slog.Logger
}

type OpenAIProviderOption func(*OpenAIProvider)

func WithAPIKey(apiKey string) OpenAIProviderOption {
	return func(p *OpenAIProvider) {
		p.apiKey = apiKey
	}
}

func WithModel(model string) OpenAIProviderOption {
	return func(p *OpenAIProvider) {
		p.model = model
	}
}

func WithLogger(logger *slog.Logger) OpenAIProviderOption {
	return func(p *OpenAIProvider) {
		p.logger = logger
	}
}

type ProviderResponseFormat struct {
	Name         string
	Description  string
	Schema       string
	SystemPrompt string
	UserPrompt   string
}
