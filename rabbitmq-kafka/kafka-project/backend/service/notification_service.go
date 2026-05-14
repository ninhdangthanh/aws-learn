package service

import (
	"context"
	"encoding/json"
	"log"

	"github.com/kafka-commerce/backend/kafka"
	"github.com/kafka-commerce/backend/models"
)

type NotificationService struct {
	consumer   *kafka.Consumer
	instanceID string
}

func NewNotificationService(brokers []string, instanceID string) *NotificationService {
	consumer := kafka.NewConsumer(brokers, "orders", "notification-service", 0)
	return &NotificationService{
		consumer:   consumer,
		instanceID: instanceID,
	}
}

func (ns *NotificationService) Start(ctx context.Context) {
	log.Printf("[NOTIFICATION SERVICE - %s] started listening to 'orders' topic", ns.instanceID)

	for {
		select {
		case <-ctx.Done():
			ns.consumer.Close()
			return
		default:
			msg, err := ns.consumer.ReadMessage(ctx)
			if err != nil {
				log.Printf("[NOTIFICATION SERVICE - %s] Error reading message: %v", ns.instanceID, err)
				continue
			}

			var event models.Event
			if err := json.Unmarshal(msg.Value, &event); err != nil {
				log.Printf("[NOTIFICATION SERVICE - %s] Error unmarshaling event: %v", ns.instanceID, err)
				continue
			}

			if event.EventType == "order.created" {
				log.Printf("[NOTIFICATION SERVICE - %s] Sending notification for order on partition %d offset %d: %s", ns.instanceID, msg.Partition, msg.Offset, event.Data)
				log.Printf("[NOTIFICATION SERVICE - %s] Email sent to customer for order: %s", ns.instanceID, event.Data)
			}

			ns.consumer.CommitMessage(ctx, msg)
		}
	}
}
