package httpserver

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/ninhdangthanh/rag-chatbot-ai-be/internal/config"
)

type Server struct {
	httpServer *http.Server
}

func New(cfg config.Config) *Server {
	router := gin.New()
	router.Use(gin.Logger(), gin.Recovery())

	api := router.Group("/api/v1")
	api.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status":    "ok",
			"service":   cfg.App.Name,
			"timestamp": time.Now().UTC().Format(time.RFC3339),
		})
	})

	return &Server{
		httpServer: &http.Server{
			Addr:         cfg.App.Address(),
			Handler:      router,
			ReadTimeout:  cfg.App.ReadTimeout,
			WriteTimeout: cfg.App.WriteTimeout,
		},
	}
}

func (s *Server) Start() error {
	return s.httpServer.ListenAndServe()
}

func (s *Server) Shutdown(ctx context.Context) error {
	return s.httpServer.Shutdown(ctx)
}
