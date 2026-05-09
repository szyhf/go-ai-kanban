package main

import (
	"flag"
	"log/slog"
	"os"

	"github.com/xuzhiping7/ai-kanban/internal/app"
	"github.com/xuzhiping7/ai-kanban/internal/config"
)

func main() {
	flag.Parse()

	cfg, err := config.Load()
	if err != nil {
		slog.Error("failed to load config", "error", err)
		os.Exit(1)
	}

	application, err := app.New(cfg)
	if err != nil {
		slog.Error("failed to initialize app", "error", err)
		os.Exit(1)
	}
	defer application.Close()

	slog.Info("vibe-kanban starting",
		"addr", cfg.Address(),
		"mode", cfg.Server.Mode,
		"db_driver", cfg.Database.Driver,
	)

	if err := application.Run(); err != nil {
		slog.Error("application error", "error", err)
		os.Exit(1)
	}
}
