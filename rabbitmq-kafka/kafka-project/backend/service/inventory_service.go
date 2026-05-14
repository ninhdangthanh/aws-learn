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
	groupID    string
}

func NewInventoryService(brokers []string, instanceID string) *InventoryService {
	groupID := "inventory-service"
	consumer := kafka.NewConsumer(brokers, "orders", groupID, 0)

	// Register with the consumer registry
	kafka.GetRegistry().Register(groupID, instanceID)

	return &InventoryService{
		consumer:   consumer,
		instanceID: instanceID,
		groupID:    groupID,
	}
}

func (is *InventoryService) Start(ctx context.Context) {
	log.Printf("[INVENTORY SERVICE - %s] started listening to 'orders' topic", is.instanceID)

	defer func() {
		is.consumer.Close()
		kafka.GetRegistry().Unregister(is.groupID, is.instanceID)
		log.Printf("[INVENTORY SERVICE - %s] stopped", is.instanceID)
	}()

	for {
		select {
		case <-ctx.Done():
			return
		default:
			msg, err := is.consumer.ReadMessage(ctx)
			if err != nil {
				if ctx.Err() != nil {
					return
				}
				log.Printf("[INVENTORY SERVICE - %s] Error reading message: %v", is.instanceID, err)
				continue
			}

			// Track partition assignment and message processing
			kafka.GetRegistry().RecordMessage(is.groupID, is.instanceID, msg.Partition)

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
