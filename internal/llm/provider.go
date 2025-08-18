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

	var schemaObj map[string]any
	if err := json.Unmarshal([]byte(AnalyzerSchema), &schemaObj); err != nil {
		return "", fmt.Errorf("invalid analyzer schema json: %w", err)
	}
	schemaParam := openai.ResponseFormatJSONSchemaJSONSchemaParam{
		Name:        "analyzer_plan",
		Description: openai.String("Workout analyzer plan"),
		Schema:      schemaObj, // must be an object, not a string
		Strict:      openai.Bool(true),
	}

	chat, err := p.Client.Chat.Completions.New(ctx, openai.ChatCompletionNewParams{
		Messages: []openai.ChatCompletionMessageParamUnion{
			openai.UserMessage(userPrompt),
			openai.SystemMessage(systemPrompt),
		}, ResponseFormat: openai.ChatCompletionNewParamsResponseFormatUnion{
			OfJSONSchema: &openai.ResponseFormatJSONSchemaParam{
				JSONSchema: schemaParam,
			},
		},
		// only certain models can perform structured outputs
		Model: openai.ChatModelGPT4o2024_08_06,
	})
	if err != nil {
		return "", err
	}

	// Extract assistant message content as a plain string
	var s string
	if len(chat.Choices) > 0 {
		contentAny := any(chat.Choices[0].Message.Content)
		switch v := contentAny.(type) {
		case string:
			s = v
		case *string:
			if v != nil {
				s = *v
			}
		default:
			// Fallback: marshal the message and try to pull text fields
			var m map[string]any
			if b, err := json.Marshal(chat.Choices[0].Message); err == nil {
				if err2 := json.Unmarshal(b, &m); err2 == nil {
					if sc, ok := m["content"].(string); ok {
						s = sc
					} else if arr, ok := m["content"].([]any); ok {
						var bld strings.Builder
						for _, it := range arr {
							if mm, ok := it.(map[string]any); ok {
								if txt, ok := mm["text"].(string); ok {
									bld.WriteString(txt)
								}
								if inner, ok2 := mm["text"].(map[string]any); ok2 {
									if val, ok3 := inner["value"].(string); ok3 {
										bld.WriteString(val)
									}
								}
							}
						}
						s = bld.String()
					}
				}
			}
		}
	}
	s = strings.TrimSpace(s)
	if s == "" {
		return "", fmt.Errorf("no message content")
	}
	return s, nil
}
