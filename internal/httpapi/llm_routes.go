package httpapi

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"

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

		plan, traces, err := cli.AnalyzeWithDebug(context.Background(), in)
		if os.Getenv("LLM_DEBUG") != "" {
			for _, t := range traces {
				log.Printf("[LLM_DEBUG] phase=%s\nSYSTEM:\n%s\n\nUSER:\n%s\n\nRAW:\n%s\n", t.Phase, trimForLog(t.System, 2000), trimForLog(t.User, 4000), trimForLog(t.Raw, 4000))
			}
		}
		if err != nil {
			return c.Status(http.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
		}
		return c.JSON(plan)
	})
}

func trimForLog(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "\n... [truncated] ...\n" + s[len(s)-min(200, len(s)):] // show head and tail
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
