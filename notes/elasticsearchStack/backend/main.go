package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"

	"es-sync-backend/internal/config"
	"es-sync-backend/internal/esclient"
	"es-sync-backend/internal/handler"
	"es-sync-backend/internal/outbox"
	"es-sync-backend/internal/store"
)

//go:generate echo "migrations embedded via os.ReadFile at startup"

func main() {
	_ = godotenv.Load()
	cfg := config.Load()

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// --- Postgres (source of truth) ---
	st, err := store.New(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("postgres: %v", err)
	}
	defer st.Close()

	ddl, err := os.ReadFile("migrations/001_init.sql")
	if err != nil {
		log.Fatalf("đọc migration: %v", err)
	}
	if err := st.Migrate(ctx, string(ddl)); err != nil {
		log.Fatalf("migrate: %v", err)
	}

	// --- Elasticsearch (secondary store cho search) ---
	es := esclient.New(cfg.ESURL, cfg.ESAlias)
	if err := es.Ping(ctx); err != nil {
		log.Printf("[warn] ES chưa sẵn sàng: %v (server vẫn chạy, worker sẽ retry)", err)
	}
	if err := es.EnsureIndexAlias(ctx); err != nil {
		log.Printf("[warn] EnsureIndexAlias: %v (thử lại khi ES lên)", err)
	}

	// --- Outbox worker (chỉ ở write_mode=outbox) ---
	if cfg.RunWorker && cfg.WriteMode != "dual" {
		poll, err := time.ParseDuration(cfg.WorkerPoll)
		if err != nil {
			poll = time.Second
		}
		w := outbox.New(st, es, poll)
		go w.Run(ctx)
	} else {
		log.Printf("[outbox] worker TẮT (write_mode=%s) — dual-write demo", cfg.WriteMode)
	}

	// --- HTTP ---
	gin.SetMode(gin.ReleaseMode)
	r := gin.Default()
	r.Use(cors.New(cors.Config{
		AllowOrigins: []string{cfg.CORSOrigin},
		AllowMethods: []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowHeaders: []string{"Content-Type"},
	}))
	handler.New(st, es, cfg.WriteMode).Register(r)

	srv := &http.Server{Addr: cfg.Port, Handler: r}
	go func() {
		log.Printf("listening on %s (write_mode=%s, es_alias=%s)", cfg.Port, cfg.WriteMode, cfg.ESAlias)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("server: %v", err)
		}
	}()

	<-ctx.Done()
	log.Printf("shutting down...")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = srv.Shutdown(shutdownCtx)
}
