package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"

	"github.com/ninhdang/jwt-auth/internal/auth/handler"
	"github.com/ninhdang/jwt-auth/internal/auth/repository"
	"github.com/ninhdang/jwt-auth/internal/auth/service"
	"github.com/ninhdang/jwt-auth/migrations"
	"github.com/ninhdang/jwt-auth/pkg/config"
	"github.com/ninhdang/jwt-auth/pkg/jwt"
	"github.com/ninhdang/jwt-auth/pkg/postgres"
	"github.com/ninhdang/jwt-auth/pkg/redis"
)

func main() {
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo})))

	if err := run(); err != nil {
		slog.Error("server exited with error", "err", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	isDev := cfg.AppEnv == "development"

	// --- Infrastructure ---
	db, err := postgres.New(cfg.DBDsn, isDev)
	if err != nil {
		return err
	}
	if err := postgres.Migrate(db, migrations.FS); err != nil {
		return err
	}
	slog.Info("postgres connected & migrated")

	redisClient, err := redis.New(cfg.RedisAddr, cfg.RedisPassword, cfg.RedisDB)
	if err != nil {
		return err
	}
	defer redisClient.Close()
	slog.Info("redis connected", "addr", cfg.RedisAddr)

	// --- Wiring: repository -> service -> handler ---
	jwtManager := jwt.NewManager(cfg.JWTSecret, cfg.JWTIssuer, cfg.AccessTokenTTL, cfg.RefreshTTL)

	userRepo := repository.NewUserRepository(db)
	refreshRepo := repository.NewRefreshRepository(db)
	tokenStore := repository.NewTokenStore(redisClient)

	authService := service.NewAuthService(userRepo, refreshRepo, tokenStore, jwtManager)
	tokenGuard := service.NewTokenGuard(userRepo, tokenStore)
	authHandler := handler.NewAuthHandler(authService, jwtManager, tokenGuard)

	// --- HTTP ---
	if !isDev {
		gin.SetMode(gin.ReleaseMode)
	}
	router := gin.New()
	router.Use(gin.Logger(), gin.Recovery())
	router.Use(cors.New(cors.Config{
		AllowOrigins:     cfg.CORSOrigins,
		AllowMethods:     []string{http.MethodGet, http.MethodPost, http.MethodPut, http.MethodDelete, http.MethodOptions},
		AllowHeaders:     []string{"Origin", "Content-Type", "Authorization"},
		AllowCredentials: true,
		MaxAge:           12 * time.Hour,
	}))

	router.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok", "time": time.Now()})
	})

	authHandler.RegisterRoutes(router.Group("/api/v1"))

	srv := &http.Server{
		Addr:              ":" + cfg.AppPort,
		Handler:           router,
		ReadHeaderTimeout: 10 * time.Second,
	}

	// Graceful shutdown: đợi request đang chạy hoàn tất trước khi thoát.
	errCh := make(chan error, 1)
	go func() {
		slog.Info("http server listening", "port", cfg.AppPort)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	select {
	case err := <-errCh:
		return err
	case <-quit:
		slog.Info("shutting down...")
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		return srv.Shutdown(ctx)
	}
}
