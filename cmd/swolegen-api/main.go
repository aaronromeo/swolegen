package main

import (
	"log"
	"log/slog"
	"os"

	"github.com/aaronromeo/swolegen/internal/config"
	"github.com/aaronromeo/swolegen/internal/httpapi"
)

func main() {
	cfg, err := config.LoadConfig()
	if err != nil {
		log.Fatal(err)
	}

	programLevel := slog.LevelInfo
	if cfg.Debug {
		programLevel = slog.LevelDebug
	}

	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: programLevel}))
	slog.SetDefault(logger)

	app := httpapi.NewServer(cfg, logger)
	log.Printf("listening on %s", cfg.Addr)
	if err := app.Listen(cfg.Addr); err != nil {
		log.Fatal(err)
	}
}
