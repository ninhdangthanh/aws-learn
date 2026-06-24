package worker

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/hibiken/asynq"

	"github.com/ninhdangthanh/rag-chatbot-ai-be/internal/config"
	"github.com/ninhdangthanh/rag-chatbot-ai-be/internal/tasks"
)

type TaskDistributor interface {
	EnqueueParseDocument(ctx context.Context, documentID uuid.UUID, filePath string) error
	Close() error
}

type RedisTaskDistributor struct {
	client    *asynq.Client
	queueName string
}

func NewRedisTaskDistributor(cfg config.Config) *RedisTaskDistributor {
	client := asynq.NewClient(asynq.RedisClientOpt{
		Addr:     cfg.Redis.Addr,
		Password: cfg.Redis.Password,
		DB:       cfg.Redis.DB,
	})

	return &RedisTaskDistributor{
		client:    client,
		queueName: cfg.Asynq.QueueName,
	}
}

func (d *RedisTaskDistributor) EnqueueParseDocument(ctx context.Context, documentID uuid.UUID, filePath string) error {
	task, err := tasks.NewDocumentParseTask(documentID, filePath)
	if err != nil {
		return err
	}

	if _, err := d.client.EnqueueContext(ctx, task, tasks.EnqueueOptions(d.queueName)...); err != nil {
		return fmt.Errorf("enqueue parse document task: %w", err)
	}

	return nil
}

func (d *RedisTaskDistributor) Close() error {
	return d.client.Close()
}
