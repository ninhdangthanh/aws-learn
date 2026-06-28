package main

import (
	"context"
	"log"

	"github.com/hibiken/asynq"

	"github.com/ninhdangthanh/rag-chatbot-ai-be/internal/chunker"
	"github.com/ninhdangthanh/rag-chatbot-ai-be/internal/config"
	"github.com/ninhdangthanh/rag-chatbot-ai-be/internal/embedding"
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
	vectorRepo, err := repository.NewVectorRepository(cfg.Qdrant, cfg.OpenAI.EmbeddingDimensions)
	if err != nil {
		log.Fatalf("create qdrant repository: %v", err)
	}
	if err := vectorRepo.EnsureCollection(context.Background()); err != nil {
		log.Fatalf("ensure qdrant collection: %v", err)
	}

	taskDistributor := worker.NewRedisTaskDistributor(cfg)
	defer taskDistributor.Close()

	// Gemini có embedding model riêng; nếu muốn đi theo Google/Gemini stack,
	// xem embedding.NewGeminiClient trong internal/embedding/gemini.go.
	embedder := embedding.NewOpenAIClient(embedding.OpenAIConfig{
		APIKey:     cfg.OpenAI.APIKey,
		Model:      cfg.OpenAI.EmbeddingModel,
		Dimensions: cfg.OpenAI.EmbeddingDimensions,
	})

	parseService := service.NewParseChunkService(
		parser.NewPDFParser(),
		chunker.New(cfg.Chunking.ChunkSizeTokens, cfg.Chunking.ChunkOverlapTokens),
		documentRepo,
		chunkRepo,
	)

	taskProcessor := worker.NewTaskProcessor(
		parseService,
		documentRepo,
		worker.WithTaskDistributor(taskDistributor),
	)
	embeddingTaskProcessor := worker.NewEmbeddingTaskProcessor(documentRepo, chunkRepo, embedder, vectorRepo)
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
	mux.HandleFunc(tasks.TypeDocumentEmbed, embeddingTaskProcessor.ProcessDocumentEmbedTask)

	log.Printf(
		"worker starting (redis=%s, postgres=%s:%d, qdrant_collection=%s, queue=%s)",
		cfg.Redis.Addr,
		cfg.Postgres.Host,
		cfg.Postgres.Port,
		cfg.Qdrant.Collection,
		cfg.Asynq.QueueName,
	)

	if err := server.Run(mux); err != nil {
		log.Fatalf("start asynq server: %v", err)
	}
}
