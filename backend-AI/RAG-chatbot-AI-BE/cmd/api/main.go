package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os/signal"
	"syscall"

	"github.com/ninhdangthanh/rag-chatbot-ai-be/internal/config"
	"github.com/ninhdangthanh/rag-chatbot-ai-be/internal/httpserver"
	"github.com/ninhdangthanh/rag-chatbot-ai-be/internal/postgres"
	"github.com/ninhdangthanh/rag-chatbot-ai-be/internal/worker"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("load config: %v", err)
	}

	db, err := postgres.Open(context.Background(), cfg.Postgres)
	if err != nil {
		log.Fatalf("open postgres: %v", err)
	}

	distributor := worker.NewRedisTaskDistributor(cfg)
	defer distributor.Close()

	server := httpserver.New(cfg, db, distributor)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	errCh := make(chan error, 1)
	go func() {
		errCh <- server.Start()
	}()

	log.Printf("api server starting on %s", cfg.App.Address())

	select {
	case err := <-errCh:
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("api server stopped unexpectedly: %v", err)
		}
	case <-ctx.Done():
		log.Printf("shutdown signal received")
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.App.ShutdownTimeout)
	defer cancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Fatalf("shutdown api server: %v", err)
	}

	log.Printf("api server stopped")
}
