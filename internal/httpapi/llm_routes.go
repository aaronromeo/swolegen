package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"strconv"

	"github.com/aaronromeo/swolegen/internal/llm"
	"github.com/aaronromeo/swolegen/internal/llm/provider"
	"github.com/gofiber/fiber/v2"
)

func registerLLM(app *fiber.App) {
	app.Post("/llm/analyze", func(c *fiber.Ctx) error {
		var in llm.AnalyzerInputs
		if err := json.Unmarshal(c.Body(), &in); err != nil {
			return c.Status(http.StatusBadRequest).JSON(fiber.Map{"error": "invalid json: " + err.Error()})
		}

		llmProvider, err := provider.NewOpenAIProvider(
			provider.WithAPIKey(os.Getenv("OPENAI_API_KEY")),
			provider.WithModel(os.Getenv("LLM_MODEL_ANALYZER")),
		)
		if err != nil {
			return c.Status(http.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}
		retries, err := strconv.Atoi(os.Getenv("LLM_RETRIES"))
		if err != nil {
			return c.Status(http.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}
		cli, err := llm.New(
			llm.WithRetries(retries),
			llm.WithProvider(llmProvider),
		)
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
		var in llm.AnalyzerPlan
		if err := json.Unmarshal(c.Body(), &in); err != nil {
			return c.Status(http.StatusBadRequest).JSON(fiber.Map{"error": "invalid json: " + err.Error()})
		}

		cli, err := llm.New()
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
