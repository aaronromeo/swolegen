package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/openai/openai-go/v2"
	"github.com/openai/openai-go/v2/option"
)

func NewOpenAIProvider(opts ...OpenAIProviderOption) (*OpenAIProvider, error) {
	p := &OpenAIProvider{model: DefaultModel}
	for _, opt := range opts {
		opt(p)
	}
	cli := openai.NewClient(option.WithAPIKey(p.apiKey))
	p.Client = cli

	return p, nil
}

func (p *OpenAIProvider) Validate() error {
	if p.apiKey == "" {
		return fmt.Errorf("api key not set")
	}
	return nil
}

func (p *OpenAIProvider) Complete(ctx context.Context, prf ProviderResponseFormat) (string, error) {
	params := openai.ChatCompletionNewParams{
		Messages: []openai.ChatCompletionMessageParamUnion{
			openai.SystemMessage(prf.SystemPrompt),
			openai.UserMessage(prf.UserPrompt),
		},
		Model: openai.ChatModel(p.model),
	}

	var schemaObj map[string]any
	if err := json.Unmarshal([]byte(prf.Schema), &schemaObj); err == nil {
		params.ResponseFormat = openai.ChatCompletionNewParamsResponseFormatUnion{
			OfJSONSchema: &openai.ResponseFormatJSONSchemaParam{
				JSONSchema: openai.ResponseFormatJSONSchemaJSONSchemaParam{
					Name:        prf.Name,
					Description: openai.String(prf.Description),
					Schema:      schemaObj,
					Strict:      openai.Bool(true),
				},
			},
		}
	}
	chat, err := p.Client.Chat.Completions.New(ctx, params)
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
