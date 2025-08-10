package llm

import (
	"context"
	"errors"
)

type AnalyzerInputs struct {
	// TODO: fill with fields you pass to analyzer prompt
}

type AnalyzerPlan struct {
	// TODO: mirror analyzer JSON
}

type Client struct{}

func New() *Client { return &Client{} }

func (c *Client) Analyze(ctx context.Context, in AnalyzerInputs) (AnalyzerPlan, error) {
	return AnalyzerPlan{}, errors.New("not implemented")
}

func (c *Client) Generate(ctx context.Context, plan AnalyzerPlan) ([]byte, error) {
	return nil, errors.New("not implemented")
}
