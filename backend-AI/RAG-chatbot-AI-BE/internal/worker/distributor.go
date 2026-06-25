package worker

import (
	"context"
	"fmt"
	"log"

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

	log.Printf(
		"enqueueing parse document task: type=%s document_id=%s file_path=%s queue=%s",
		task.Type(),
		documentID,
		filePath,
		d.queueName,
	)

	info, err := d.client.EnqueueContext(ctx, task, tasks.EnqueueOptions(d.queueName)...)
	if err != nil {
		log.Printf(
			"failed to enqueue parse document task: type=%s document_id=%s queue=%s err=%v",
			task.Type(),
			documentID,
			d.queueName,
			err,
		)
		return fmt.Errorf("enqueue parse document task: %w", err)
	}

	log.Printf(
		"enqueued parse document task: id=%s type=%s document_id=%s queue=%s state=%s",
		info.ID,
		info.Type,
		documentID,
		info.Queue,
		info.State,
	)

	return nil
}

func (d *RedisTaskDistributor) Close() error {
	return d.client.Close()
}
