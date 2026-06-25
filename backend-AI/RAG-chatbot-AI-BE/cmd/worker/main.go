package main

import (
	"context"
	"log"

	"github.com/hibiken/asynq"

	"github.com/ninhdangthanh/rag-chatbot-ai-be/internal/chunker"
	"github.com/ninhdangthanh/rag-chatbot-ai-be/internal/config"
	"github.com/ninhdangthanh/rag-chatbot-ai-be/internal/parser"
	"github.com/ninhdangthanh/rag-chatbot-ai-be/internal/postgres"
	"github.com/ninhdangthanh/rag-chatbot-ai-be/internal/repository"
	"github.com/ninhdangthanh/rag-chatbot-ai-be/internal/service"
	"github.com/ninhdangthanh/rag-chatbot-ai-be/internal/tasks"
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

	documentRepo := repository.NewDocumentRepository(db)
	chunkRepo := repository.NewChunkRepository(db)
	parseService := service.NewParseChunkService(
		parser.NewPDFParser(),
		chunker.New(cfg.Chunking.ChunkSizeTokens, cfg.Chunking.ChunkOverlapTokens),
		documentRepo,
		chunkRepo,
	)

	taskProcessor := worker.NewTaskProcessor(parseService, documentRepo)
	server := asynq.NewServer(
		asynq.RedisClientOpt{
			Addr:     cfg.Redis.Addr,
			Password: cfg.Redis.Password,
			DB:       cfg.Redis.DB,
		},
		asynq.Config{
			Concurrency: cfg.Asynq.Concurrency,
			Queues: map[string]int{
				cfg.Asynq.QueueName: 1,
			},
			ErrorHandler: asynq.ErrorHandlerFunc(func(ctx context.Context, task *asynq.Task, err error) {
				log.Printf("asynq task failed: type=%s payload=%s err=%v", task.Type(), string(task.Payload()), err)
			}),
		},
	)

	mux := asynq.NewServeMux()
	mux.HandleFunc(tasks.TypeDocumentParse, taskProcessor.ProcessDocumentParseTask)

	log.Printf(
		"worker starting (redis=%s, postgres=%s:%d, queue=%s)",
		cfg.Redis.Addr,
		cfg.Postgres.Host,
		cfg.Postgres.Port,
		cfg.Asynq.QueueName,
	)

	if err := server.Run(mux); err != nil {
		log.Fatalf("start asynq server: %v", err)
	}
}
