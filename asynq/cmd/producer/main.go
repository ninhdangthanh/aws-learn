package main

import (
	"log"
	"os"
	"time"

	"asynq/task"

	"github.com/hibiken/asynq"
)

func main() {
	redisAddr := getenv("REDIS_ADDR", "127.0.0.1:6379")

	client := asynq.NewClient(asynq.RedisClientOpt{Addr: redisAddr})
	defer client.Close()

	emailTask, err := task.NewEmailDeliveryTask(task.EmailDeliveryPayload{
		UserID:      42,
		Email:       "learner@example.com",
		TemplateID:  "welcome",
		Subject:     "Welcome to Asynq",
		RequestedBy: "producer-demo",
	})
	if err != nil {
		log.Fatalf("could not create email task: %v", err)
	}

	info, err := client.Enqueue(
		emailTask,
		asynq.Queue(task.QueueCritical),
		asynq.MaxRetry(5),
		asynq.Timeout(30*time.Second),
	)
	if err != nil {
		log.Fatalf("could not enqueue email task: %v", err)
	}
	log.Printf("enqueued email task: id=%s queue=%s", info.ID, info.Queue)

	futureEmailTask, err := task.NewEmailDeliveryTask(task.EmailDeliveryPayload{
		UserID:      100,
		Email:       "later@example.com",
		TemplateID:  "daily-summary",
		Subject:     "Tomorrow summary",
		RequestedBy: "producer-demo",
	})
	if err != nil {
		log.Fatalf("could not create scheduled email task: %v", err)
	}

	info, err = client.Enqueue(
		futureEmailTask,
		asynq.Queue(task.QueueDefault),
		asynq.ProcessIn(10*time.Second),
		asynq.Unique(1*time.Minute),
	)
	if err != nil {
		log.Fatalf("could not schedule email task: %v", err)
	}
	log.Printf("scheduled email task: id=%s queue=%s", info.ID, info.Queue)

	imageTask, err := task.NewImageResizeTask(task.ImageResizePayload{
		SourceURL: "https://example.com/assets/avatar.jpg",
		Width:     320,
		Height:    320,
		Format:    "webp",
		OwnerID:   42,
	})
	if err != nil {
		log.Fatalf("could not create image task: %v", err)
	}

	info, err = client.Enqueue(
		imageTask,
		asynq.Queue(task.QueueLow),
		asynq.MaxRetry(10),
		asynq.Timeout(3*time.Minute),
	)
	if err != nil {
		log.Fatalf("could not enqueue image task: %v", err)
	}
	log.Printf("enqueued image task: id=%s queue=%s", info.ID, info.Queue)
}

func getenv(key, fallback string) string {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	return value
}
