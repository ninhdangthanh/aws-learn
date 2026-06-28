package main

import (
	"context"
	"log"
	"os"

	"asynq/task"

	"github.com/hibiken/asynq"
)

func main() {
	redisAddr := getenv("REDIS_ADDR", "127.0.0.1:6379")

	srv := asynq.NewServer(
		asynq.RedisClientOpt{Addr: redisAddr},
		asynq.Config{
			Concurrency: 5,
			Queues: map[string]int{
				task.QueueCritical: 6,
				task.QueueDefault:  3,
				task.QueueLow:      1,
			},
			ErrorHandler: asynq.ErrorHandlerFunc(func(ctx context.Context, t *asynq.Task, err error) {
				log.Printf("task failed: type=%s payload=%s err=%v", t.Type(), string(t.Payload()), err)
			}),
		},
	)

	mux := asynq.NewServeMux()
	mux.HandleFunc(task.TypeEmailDelivery, task.HandleEmailDeliveryTask)
	mux.Handle(task.TypeImageResize, task.NewImageProcessor(task.ImageProcessorConfig{
		OutputDir: "./tmp/resized",
	}))

	log.Printf("worker is running with Redis %s", redisAddr)
	if err := srv.Run(mux); err != nil {
		log.Fatalf("could not run worker: %v", err)
	}
}

func getenv(key, fallback string) string {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	return value
}
