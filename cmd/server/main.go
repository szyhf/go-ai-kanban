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
		slog.Error("加载配置失败", "error", err)
		os.Exit(1)
	}

	application, err := app.New(cfg)
	if err != nil {
		slog.Error("初始化应用失败", "error", err)
		os.Exit(1)
	}
	defer application.Close()

	slog.Info("vibe-kanban 启动中",
		"addr", cfg.Address(),
		"mode", cfg.Server.Mode,
		"db_driver", cfg.Database.Driver,
	)

	if err := application.Run(); err != nil {
		slog.Error("应用运行错误", "error", err)
		os.Exit(1)
	}
}
