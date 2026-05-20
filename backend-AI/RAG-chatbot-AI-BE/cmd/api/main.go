package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/hibiken/asynq"
	"github.com/rs/zerolog"

	"github.com/ninhdangthanh/rag-chatbot/internal/config"
	"github.com/ninhdangthanh/rag-chatbot/internal/handler"
	"github.com/ninhdangthanh/rag-chatbot/internal/repository"
	"github.com/ninhdangthanh/rag-chatbot/internal/service"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatal(err)
	}

	logger := zerolog.New(os.Stdout).With().Timestamp().Logger()

	db, err := repository.NewPostgres(cfg.PostgresDSN)
	if err != nil {
		logger.Fatal().Err(err).Msg("failed to connect to postgres")
	}
	defer db.Close(context.Background())

	qdrantRepo, err := repository.NewQdrant(cfg.QdrantAddr, cfg.QdrantCollection)
	if err != nil {
		logger.Fatal().Err(err).Msg("failed to connect to qdrant")
	}

	asynqClient := asynq.NewClient(asynq.RedisClientOpt{Addr: cfg.RedisAddr})
	defer asynqClient.Close()

	documentService := service.NewDocumentService(db, qdrantRepo, asynqClient, cfg)
	ingestService := service.NewIngestionService(db, qdrantRepo, cfg)
	embedService := service.NewEmbeddingService(cfg)
	retrievalService := service.NewRetrievalService(qdrantRepo, cfg)
	chatService := service.NewChatService(db, ingestService, retrievalService, embedService, cfg)

	router := gin.New()
	router.Use(gin.Recovery())

	h := handler.NewHandler(documentService, retrievalService, chatService, cfg)
	router.POST("/api/v1/documents", h.UploadDocument)
	router.GET("/api/v1/documents", h.ListDocuments)
	router.GET("/api/v1/documents/:id", h.GetDocument)
	router.DELETE("/api/v1/documents/:id", h.DeleteDocument)
	router.POST("/api/v1/search", h.Search)
	router.POST("/api/v1/chat", h.Chat)
	router.GET("/api/v1/health", h.Health)

	server := &http.Server{
		Addr:              fmt.Sprintf(":%d", cfg.Port),
		Handler:           router,
		ReadHeaderTimeout: 10 * time.Second,
	}

	logger.Info().Msgf("API server listening on %s", server.Addr)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		logger.Fatal().Err(err).Msg("server failed")
	}
}
