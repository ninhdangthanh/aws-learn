package httpserver

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/ninhdangthanh/rag-chatbot-ai-be/internal/config"
	"github.com/ninhdangthanh/rag-chatbot-ai-be/internal/embedding"
	"github.com/ninhdangthanh/rag-chatbot-ai-be/internal/handler"
	"github.com/ninhdangthanh/rag-chatbot-ai-be/internal/llm"
	"github.com/ninhdangthanh/rag-chatbot-ai-be/internal/repository"
	"github.com/ninhdangthanh/rag-chatbot-ai-be/internal/service"
	"github.com/ninhdangthanh/rag-chatbot-ai-be/internal/worker"
)

type Server struct {
	httpServer *http.Server
}

func New(cfg config.Config, db *gorm.DB, distributor worker.TaskDistributor) (*Server, error) {
	router := gin.New()
	router.Use(gin.Logger(), gin.Recovery())
	router.MaxMultipartMemory = cfg.Upload.MaxFileSizeBytes

	vectorRepo, err := repository.NewVectorRepository(cfg.Qdrant, cfg.OpenAI.EmbeddingDimensions)
	if err != nil {
		return nil, fmt.Errorf("create vector repository: %w", err)
	}
	if err := vectorRepo.EnsureCollection(context.Background()); err != nil {
		return nil, fmt.Errorf("ensure qdrant collection: %w", err)
	}

	documentHandler := handler.NewDocumentHandler(
		repository.NewDocumentRepository(db),
		distributor,
		cfg.Upload.Dir,
		cfg.Upload.MaxFileSizeBytes,
	)
	embedder := embedding.NewOpenAIClient(embedding.OpenAIConfig{
		APIKey:     cfg.OpenAI.APIKey,
		Model:      cfg.OpenAI.EmbeddingModel,
		Dimensions: cfg.OpenAI.EmbeddingDimensions,
	})
	searchService := service.NewSearchService(embedder, vectorRepo)
	searchHandler := handler.NewSearchHandler(searchService)
	chatRepo := repository.NewChatRepository(db)
	llmClient := llm.NewOpenAIClient(llm.OpenAIConfig{
		APIKey: cfg.OpenAI.APIKey,
		Model:  cfg.OpenAI.ChatModel,
	})
	chatHandler := handler.NewChatHandler(
		service.NewRAGChatService(searchService, llmClient, chatRepo),
	)
	chatSessionHandler := handler.NewChatSessionHandler(chatRepo)

	api := router.Group("/api/v1")
	api.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status":    "ok",
			"service":   cfg.App.Name,
			"timestamp": time.Now().UTC().Format(time.RFC3339),
		})
	})
	api.POST("/documents", documentHandler.Upload)
	api.GET("/documents/:id", documentHandler.GetStatus)
	api.POST("/search", searchHandler.Search)
	api.POST("/chat", chatHandler.Chat)
	api.GET("/chat/sessions", chatSessionHandler.ListSessions)
	api.GET("/chat/sessions/:id/messages", chatSessionHandler.ListMessages)

	return &Server{
		httpServer: &http.Server{
			Addr:         cfg.App.Address(),
			Handler:      router,
			ReadTimeout:  cfg.App.ReadTimeout,
			WriteTimeout: cfg.App.WriteTimeout,
		},
	}, nil
}

func (s *Server) Start() error {
	return s.httpServer.ListenAndServe()
}

func (s *Server) Shutdown(ctx context.Context) error {
	return s.httpServer.Shutdown(ctx)
}
