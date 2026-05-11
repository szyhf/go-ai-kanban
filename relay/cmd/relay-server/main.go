package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/xuzhiping7/ai-kanban/relay"
	"github.com/xuzhiping7/ai-kanban/relay/internal/api"
	"github.com/xuzhiping7/ai-kanban/relay/internal/auth"
	"github.com/xuzhiping7/ai-kanban/relay/internal/store"
)

func main() {
	cfg := relay.ConfigFromEnv()

	db, err := store.NewStore(cfg.DBPath)
	if err != nil {
		log.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	jwtMgr := auth.NewJWTManager(cfg.JWTSecret)
	sessions := auth.NewSessionStore()
	stopCleaner := sessions.StartCleaner()
	defer close(stopCleaner)

	srv := api.NewServer(db, jwtMgr, sessions)
	defer srv.Close()

	addr := fmt.Sprintf("%s:%d", cfg.Host, cfg.Port)
	log.Printf("Relay server starting on %s (db: %s)", addr, cfg.DBPath)

	httpServer := &http.Server{
		Addr:    addr,
		Handler: srv.Handler(),
	}

	go func() {
		if err := httpServer.ListenAndServe(); err != nil && err.Error() != "http: Server closed" {
			log.Fatalf("HTTP server error: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down relay server...")
	httpServer.Close()
	log.Println("Server stopped.")
}
