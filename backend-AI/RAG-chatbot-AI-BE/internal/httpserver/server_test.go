package httpserver

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/ninhdangthanh/rag-chatbot-ai-be/internal/config"
)

func TestHealthEndpoint(t *testing.T) {
	cfg := config.Config{
		App: config.AppConfig{
			Name:            "rag-chatbot-ai-backend",
			Env:             "test",
			Host:            "127.0.0.1",
			Port:            8099,
			ReadTimeout:     5 * time.Second,
			WriteTimeout:    5 * time.Second,
			ShutdownTimeout: 5 * time.Second,
		},
		Redis: config.RedisConfig{Addr: "localhost:6379"},
		Qdrant: config.QdrantConfig{
			URL: "http://localhost:6333",
		},
	}

	server := New(cfg)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/health", nil)
	rec := httptest.NewRecorder()

	server.httpServer.Handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
}
