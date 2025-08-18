package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	openai "github.com/openai/openai-go/v2"
	"github.com/openai/openai-go/v2/option"
)

// Provider defines the minimal interface for LLM completion.
type Provider interface {
	Complete(ctx context.Context, systemPrompt string, userPrompt string) (string, error)
}

// OpenAIProvider implements Provider using the official openai-go client.
type OpenAIProvider struct {
	Client openai.Client
	Model  string
}

func NewOpenAIProviderFromEnv() (*OpenAIProvider, error) {
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		return nil, fmt.Errorf("OPENAI_API_KEY not set")
	}
	model := os.Getenv("LLM_MODEL_ANALYZER")
	if model == "" {
		model = "gpt-4.1-mini"
	}
	cli := openai.NewClient(option.WithAPIKey(apiKey))
	return &OpenAIProvider{Client: cli, Model: model}, nil
}

func (p *OpenAIProvider) Complete(ctx context.Context, systemPrompt string, userPrompt string) (string, error) {
	// Use the Responses endpoint via openai-go. Construct a generic payload.
	// We concatenate system + user as the "input" per our API design.
	params := map[string]any{
		"model": p.Model,
		"input": systemPrompt + "\n\n" + userPrompt,
	}
	var result map[string]any
	if err := p.Client.Post(ctx, "/responses", params, &result); err != nil {
		return "", err
	}
	// Prefer the convenience field if present
	if s, ok := result["output_text"].(string); ok && s != "" {
		return s, nil
	}
	// Otherwise, dig into output[*].content[*].text{,value}
	if out, ok := result["output"].([]any); ok {
		var bld strings.Builder
		for _, item := range out {
			m, ok := item.(map[string]any)
			if !ok {
				continue
			}
			content, ok := m["content"].([]any)
			if !ok {
				continue
			}
			for _, c := range content {
				cm, ok := c.(map[string]any)
				if !ok {
					continue
				}
				// type might be "output_text" or "text"
				if t, _ := cm["type"].(string); t == "output_text" || t == "text" || t == "message" {
					if s, ok := cm["text"].(string); ok {
						bld.WriteString(s)
						bld.WriteString("\n")
						continue
					}
					if tm, ok := cm["text"].(map[string]any); ok {
						if v, ok := tm["value"].(string); ok {
							bld.WriteString(v)
							bld.WriteString("\n")
						}
					}
				}
			}
		}
		if s := strings.TrimSpace(bld.String()); s != "" {
			return s, nil
		}
	}
	// Fallback: stringify the whole response if specific field missing.
	raw, err := json.Marshal(result)
	if err != nil {
		return "", fmt.Errorf("openai: unexpected response shape and marshal failed: %w", err)
	}
	return string(raw), nil
}
