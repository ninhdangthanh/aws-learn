package handler

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"

	"github.com/ninhdangthanh/rag-chatbot-ai-be/internal/config"
)

type HealthVectorStore interface {
	Health(ctx context.Context) error
}

type HealthHandler struct {
	cfg         config.Config
	db          *gorm.DB
	vectorStore HealthVectorStore
}

func NewHealthHandler(cfg config.Config, db *gorm.DB, vectorStore HealthVectorStore) *HealthHandler {
	return &HealthHandler{
		cfg:         cfg,
		db:          db,
		vectorStore: vectorStore,
	}
}

func (h *HealthHandler) Health(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 2*time.Second)
	defer cancel()

	dependencies := gin.H{}
	status := http.StatusOK

	if err := h.checkPostgres(ctx); err != nil {
		dependencies["postgres"] = gin.H{"status": "down", "error": err.Error()}
		status = http.StatusServiceUnavailable
	} else {
		dependencies["postgres"] = gin.H{"status": "ok"}
	}

	if h.vectorStore != nil {
		if err := h.vectorStore.Health(ctx); err != nil {
			dependencies["qdrant"] = gin.H{"status": "down", "error": err.Error()}
			status = http.StatusServiceUnavailable
		} else {
			dependencies["qdrant"] = gin.H{"status": "ok"}
		}
	}

	if err := h.checkRedis(ctx); err != nil {
		dependencies["redis"] = gin.H{"status": "down", "error": err.Error()}
		status = http.StatusServiceUnavailable
	} else {
		dependencies["redis"] = gin.H{"status": "ok"}
	}

	c.JSON(status, gin.H{
		"status":       healthStatus(status),
		"service":      h.cfg.App.Name,
		"timestamp":    time.Now().UTC().Format(time.RFC3339),
		"dependencies": dependencies,
	})
}

func (h *HealthHandler) checkPostgres(ctx context.Context) error {
	sqlDB, err := h.db.DB()
	if err != nil {
		return err
	}
	return sqlDB.PingContext(ctx)
}

func (h *HealthHandler) checkRedis(ctx context.Context) error {
	client := redis.NewClient(&redis.Options{
		Addr:     h.cfg.Redis.Addr,
		Password: h.cfg.Redis.Password,
		DB:       h.cfg.Redis.DB,
	})
	defer client.Close()

	return client.Ping(ctx).Err()
}

func healthStatus(status int) string {
	if status == http.StatusOK {
		return "ok"
	}
	return "degraded"
}
