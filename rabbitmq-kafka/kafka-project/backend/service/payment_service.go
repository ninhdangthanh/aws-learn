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
}

func NewPaymentService(brokers []string, instanceID string) *PaymentService {
	consumer := kafka.NewConsumer(brokers, "orders", "payment-service", 0)
	return &PaymentService{
		consumer:   consumer,
		instanceID: instanceID,
	}
}

func (ps *PaymentService) Start(ctx context.Context) {
	log.Printf("[PAYMENT SERVICE - %s] started listening to 'orders' topic", ps.instanceID)

	for {
		select {
		case <-ctx.Done():
			ps.consumer.Close()
			return
		default:
			msg, err := ps.consumer.ReadMessage(ctx)
			if err != nil {
				log.Printf("[PAYMENT SERVICE - %s] Error reading message: %v", ps.instanceID, err)
				continue
			}

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
