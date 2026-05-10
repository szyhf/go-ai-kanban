package app

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/xuzhiping7/ai-kanban/internal/config"
	"github.com/xuzhiping7/ai-kanban/internal/database"
	"github.com/xuzhiping7/ai-kanban/internal/executor"
	"github.com/xuzhiping7/ai-kanban/internal/git"
	"github.com/xuzhiping7/ai-kanban/internal/handler"
	"github.com/xuzhiping7/ai-kanban/internal/pty"
	"github.com/xuzhiping7/ai-kanban/internal/repository"
	"github.com/xuzhiping7/ai-kanban/internal/server"
	"github.com/xuzhiping7/ai-kanban/internal/service"
)

// App is the top-level application container.
type App struct {
	Config *config.Config
	DB     *database.DB
	Logger *slog.Logger
}

// New creates a new App instance, initializing all dependencies.
func New(cfg *config.Config) (*App, error) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))

	db, err := database.Open(cfg.Database.Driver, cfg.Database.DSN)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}

	if err := db.Migrate(); err != nil {
		return nil, fmt.Errorf("run migrations: %w", err)
	}

	return &App{
		Config: cfg,
		DB:     db,
		Logger: logger,
	}, nil
}

// Run starts the application and blocks until a shutdown signal is received.
func (a *App) Run() error {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	r := server.NewRouter(a.Config, a.DB, a.Logger)

	// Wire up all services, repos, and handlers.
	gitSvc := git.NewService()
	msgStore := service.NewMsgStore()
	eventSvc := service.NewEventService(msgStore, a.DB.DB)

	repoRepo := repository.NewGitRepoRepo(a.DB.DB)
	tagRepo := repository.NewTagRepo(a.DB.DB)
	scratchRepo := repository.NewScratchRepo(a.DB.DB)
	sessionRepo := repository.NewSessionRepo(a.DB.DB)
	wsRepo := repository.NewWorkspaceRepo(a.DB.DB)
	wsRepoRepo := repository.NewWorkspaceRepoRepo(a.DB.DB)
	execRepo := repository.NewExecutionProcessRepo(a.DB.DB)
	execStateRepo := repository.NewExecutionProcessRepoStateRepo(a.DB.DB)
	attachRepo := repository.NewAttachmentRepo(a.DB.DB)
	wsAttachRepo := repository.NewWorkspaceAttachmentRepo(a.DB.DB)
	turnRepo := repository.NewCodingAgentTurnRepo(a.DB.DB)

	repoSvc := service.NewRepoService(repoRepo, gitSvc)
	filesystemSvc := service.NewFilesystemService(gitSvc)
	fileSvc := service.NewFileService("", attachRepo, wsAttachRepo)
	queueSvc := service.NewQueuedMessageService()

	// Execution infrastructure.
	processStore := executor.NewProcessStore()
	containerSvc := executor.NewContainerService(
		execRepo, execStateRepo, turnRepo, wsRepoRepo,
		gitSvc, processStore, queueSvc,
	)

	// PTY terminal service.
	ptySvc := pty.NewService()

	h := handler.NewHandler(
		repoSvc, gitSvc, filesystemSvc, fileSvc, eventSvc, msgStore, queueSvc,
		repoRepo, tagRepo, scratchRepo, sessionRepo, wsRepo, wsRepoRepo,
		execRepo, execStateRepo, attachRepo, wsAttachRepo,
		containerSvc, ptySvc, turnRepo,
	)

	// Register handler routes under /api.
	r.Route("/api", h.RegisterRoutes)

	srv := &http.Server{
		Addr:    a.Config.Address(),
		Handler: r,
		BaseContext: func(_ net.Listener) context.Context {
			return ctx
		},
	}

	go func() {
		a.Logger.Info("server starting", "addr", srv.Addr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			a.Logger.Error("server error", "error", err)
		}
	}()

	<-ctx.Done()
	a.Logger.Info("shutting down...")

	// Clean up execution processes and PTY sessions.
	processStore.KillAll()
	ptySvc.CloseAll()

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return srv.Shutdown(shutdownCtx)
}

// Close releases all resources.
func (a *App) Close() error {
	if a.DB != nil {
		return a.DB.Close()
	}
	return nil
}
