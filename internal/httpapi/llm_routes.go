package httpapi

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/aaronromeo/swolegen/internal/config"
	"github.com/aaronromeo/swolegen/internal/llm"
	"github.com/aaronromeo/swolegen/internal/llm/provider"
	"github.com/aaronromeo/swolegen/internal/llm/schemas"
	"github.com/gofiber/fiber/v2"
)

func registerLLM(app *fiber.App, cfg *config.Config, logger *slog.Logger) {
	app.Post("/llm/analyze", func(c *fiber.Ctx) error {
		var in llm.AnalyzerInputs
		if err := json.Unmarshal(c.Body(), &in); err != nil {
			return c.Status(http.StatusBadRequest).JSON(fiber.Map{"error": "invalid json: " + err.Error()})
		}

		cli, err := newLLMClient(cfg, logger)
		if err != nil {
			return c.Status(http.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}

		plan, err := cli.Analyze(context.Background(), in)
		if err != nil {
			return c.Status(http.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
		}

		return c.JSON(plan)
	})

	app.Post("/llm/generate", func(c *fiber.Ctx) error {
		var in schemas.AnalyzerV1Json
		if err := json.Unmarshal(c.Body(), &in); err != nil {
			return c.Status(http.StatusBadRequest).JSON(fiber.Map{"error": "invalid json: " + err.Error()})
		}

		cli, err := newLLMClient(cfg, logger)
		if err != nil {
			return c.Status(http.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}

		plan, err := cli.Generate(context.Background(), in)
		if err != nil {
			return c.Status(http.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
		}

		return c.JSON(plan)
	})
}

func newLLMClient(cfg *config.Config, logger *slog.Logger) (*llm.Client, error) {
	llmProvider, err := provider.NewOpenAIProvider(
		provider.WithAPIKey(cfg.OpenaiKey),
		provider.WithModel(cfg.LlmModel),
		provider.WithLogger(logger),
	)
	if err != nil {
		return nil, err
	}
	cli, err := llm.New(
		llm.WithRetries(cfg.LlmRetries),
		llm.WithProvider(llmProvider),
		llm.WithLogger(logger),
	)
	if err != nil {
		return nil, err
	}
	return cli, nil
}
