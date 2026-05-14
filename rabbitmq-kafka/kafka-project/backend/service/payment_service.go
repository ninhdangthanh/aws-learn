package service

import (
	"context"
	"encoding/json"
	"log"
	"math/rand"

	"github.com/kafka-commerce/backend/kafka"
	"github.com/kafka-commerce/backend/models"
)

type PaymentService struct {
	consumer   *kafka.Consumer
	instanceID string
	groupID    string
}

func NewPaymentService(brokers []string, instanceID string) *PaymentService {
	groupID := "payment-service"
	consumer := kafka.NewConsumer(brokers, "orders", groupID, 0)

	// Register with the consumer registry
	kafka.GetRegistry().Register(groupID, instanceID)

	return &PaymentService{
		consumer:   consumer,
		instanceID: instanceID,
		groupID:    groupID,
	}
}

func (ps *PaymentService) Start(ctx context.Context) {
	log.Printf("[PAYMENT SERVICE - %s] started listening to 'orders' topic", ps.instanceID)

	defer func() {
		ps.consumer.Close()
		kafka.GetRegistry().Unregister(ps.groupID, ps.instanceID)
		log.Printf("[PAYMENT SERVICE - %s] stopped", ps.instanceID)
	}()

	for {
		select {
		case <-ctx.Done():
			return
		default:
			msg, err := ps.consumer.ReadMessage(ctx)
			if err != nil {
				if ctx.Err() != nil {
					return
				}
				log.Printf("[PAYMENT SERVICE - %s] Error reading message: %v", ps.instanceID, err)
				continue
			}

			// Track partition assignment and message processing
			kafka.GetRegistry().RecordMessage(ps.groupID, ps.instanceID, msg.Partition)

			var event models.Event
			if err := json.Unmarshal(msg.Value, &event); err != nil {
				log.Printf("[PAYMENT SERVICE - %s] Error unmarshaling event: %v", ps.instanceID, err)
				continue
			}

			if event.EventType == "order.created" {
				log.Printf("[PAYMENT SERVICE - %s] Processing order on partition %d offset %d: %s", ps.instanceID, msg.Partition, msg.Offset, event.Data)
				// Simulate payment processing
				if rand.Float64() > 0.1 { // 90% success rate
					log.Printf("[PAYMENT SERVICE - %s] Payment processed successfully for order: %s", ps.instanceID, event.Data)
				} else {
					log.Printf("[PAYMENT SERVICE - %s] Payment failed for order: %s", ps.instanceID, event.Data)
				}
			}

			ps.consumer.CommitMessage(ctx, msg)
		}
	}
}
