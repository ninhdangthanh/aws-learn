package service

import (
	"context"
	"encoding/json"
	"log"

	"github.com/kafka-commerce/backend/kafka"
	"github.com/kafka-commerce/backend/models"
)

type InventoryService struct {
	consumer   *kafka.Consumer
	instanceID string
}

func NewInventoryService(brokers []string, instanceID string) *InventoryService {
	consumer := kafka.NewConsumer(brokers, "orders", "inventory-service", 0)
	return &InventoryService{
		consumer:   consumer,
		instanceID: instanceID,
	}
}

func (is *InventoryService) Start(ctx context.Context) {
	log.Printf("[INVENTORY SERVICE - %s] started listening to 'orders' topic", is.instanceID)

	for {
		select {
		case <-ctx.Done():
			is.consumer.Close()
			return
		default:
			msg, err := is.consumer.ReadMessage(ctx)
			if err != nil {
				log.Printf("[INVENTORY SERVICE - %s] Error reading message: %v", is.instanceID, err)
				continue
			}

			var event models.Event
			if err := json.Unmarshal(msg.Value, &event); err != nil {
				log.Printf("[INVENTORY SERVICE - %s] Error unmarshaling event: %v", is.instanceID, err)
				continue
			}

			if event.EventType == "order.created" {
				log.Printf("[INVENTORY SERVICE - %s] Processing order on partition %d offset %d: %s", is.instanceID, msg.Partition, msg.Offset, event.Data)
				log.Printf("[INVENTORY SERVICE - %s] Inventory allocated for order: %s", is.instanceID, event.Data)
			}

			is.consumer.CommitMessage(ctx, msg)
		}
	}
}
