package main

import (
	"context"
	"log/slog"
	"os"

	"max-moderation-bot/internal/app"
	"max-moderation-bot/internal/config"
	"max-moderation-bot/pkg/telemetry"

	"github.com/joho/godotenv"
)

func main() {

	_ = godotenv.Load()

	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	cfg, err := config.LoadConfig()
	if err != nil {
		logger.Error("Failed to load config", "error", err)
		os.Exit(1)
	}

	if cfg.EnableTelemetry {
		shutdown, err := telemetry.InitTracer("max-moderation-bot", os.Stderr)
		if err != nil {
			logger.Error("Failed to init telemetry", "error", err)
		} else {
			defer func() {
				if err := shutdown(context.Background()); err != nil {
					logger.Error("Failed to shutdown telemetry", "error", err)
				}
			}()
		}
	}

	application, err := app.NewApp(cfg, logger)
	if err != nil {
		logger.Error("Failed to initialize app", "error", err)
		os.Exit(1)
	}

	if err := application.Run(context.Background()); err != nil {
		logger.Error("Application error", "error", err)
		os.Exit(1)
	}
}
