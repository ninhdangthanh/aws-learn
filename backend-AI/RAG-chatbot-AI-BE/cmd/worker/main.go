package main

import (
	"context"
	"log"
	"os"

	"github.com/hibiken/asynq"
	"github.com/rs/zerolog"

	"github.com/ninhdangthanh/rag-chatbot/internal/config"
	"github.com/ninhdangthanh/rag-chatbot/internal/repository"
	"github.com/ninhdangthanh/rag-chatbot/internal/service"
	"github.com/ninhdangthanh/rag-chatbot/internal/worker"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatal(err)
	}

	logger := zerolog.New(os.Stdout).With().Timestamp().Logger()

	db, err := repository.NewPostgres(cfg.PostgresDSN)
	if err != nil {
		logger.Fatal().Err(err).Msg("failed to connect postgres")
	}
	defer db.Close(context.Background())

	qdrantRepo, err := repository.NewQdrant(cfg.QdrantAddr, cfg.QdrantCollection)
	if err != nil {
		logger.Fatal().Err(err).Msg("failed to connect qdrant")
	}

	embedService := service.NewEmbeddingService(cfg)
	ingestService := service.NewIngestionService(db, qdrantRepo, cfg)
	taskHandler := worker.NewTaskHandler(db, qdrantRepo, embedService, ingestService, cfg)

	server := asynq.NewServer(
		asynq.RedisClientOpt{Addr: cfg.RedisAddr},
		asynq.Config{Concurrency: 10},
	)

	mux := asynq.NewServeMux()
	mux.HandleFunc(worker.TaskTypeParseDocument, taskHandler.HandleParseDocumentTask)
	mux.HandleFunc(worker.TaskTypeEmbedChunks, taskHandler.HandleEmbedChunksTask)

	logger.Info().Msg("worker started")
	if err := server.Run(mux); err != nil {
		logger.Fatal().Err(err).Msg("worker server failed")
	}
}
