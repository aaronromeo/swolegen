package httpapi

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/aaronromeo/swolegen/internal/llm"
	"github.com/gofiber/fiber/v2"
)

func registerLLM(app *fiber.App) {
	app.Post("/llm/analyze", func(c *fiber.Ctx) error {
		var in llm.AnalyzerInputs
		if err := json.Unmarshal(c.Body(), &in); err != nil {
			return c.Status(http.StatusBadRequest).JSON(fiber.Map{"error": "invalid json: " + err.Error()})
		}

		cli, err := llm.NewDefault()
		if err != nil {
			return c.Status(http.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}

		plan, err := cli.Analyze(context.Background(), in)
		if err != nil {
			return c.Status(http.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
		}
		return c.JSON(plan)
	})
}
