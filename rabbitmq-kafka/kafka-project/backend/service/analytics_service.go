package service

import (
	"context"
	"encoding/json"
	"log"
	"sync/atomic"

	"github.com/kafka-commerce/backend/kafka"
	"github.com/kafka-commerce/backend/models"
)

type AnalyticsService struct {
	consumer    *kafka.Consumer
	instanceID  string
	ordersCount int64
	ordersValue float64
}

func NewAnalyticsService(brokers []string, instanceID string) *AnalyticsService {
	consumer := kafka.NewConsumer(brokers, "orders", "analytics-service", 0)
	return &AnalyticsService{
		consumer:   consumer,
		instanceID: instanceID,
	}
}

func (as *AnalyticsService) Start(ctx context.Context) {
	log.Printf("[ANALYTICS SERVICE - %s] started listening to 'orders' topic", as.instanceID)

	for {
		select {
		case <-ctx.Done():
			as.consumer.Close()
			return
		default:
			msg, err := as.consumer.ReadMessage(ctx)
			if err != nil {
				log.Printf("[ANALYTICS SERVICE - %s] Error reading message: %v", as.instanceID, err)
				continue
			}

			var event models.Event
			if err := json.Unmarshal(msg.Value, &event); err != nil {
				log.Printf("[ANALYTICS SERVICE - %s] Error unmarshaling event: %v", as.instanceID, err)
				continue
			}

			if event.EventType == "order.created" {
				atomic.AddInt64(&as.ordersCount, 1)
				log.Printf("[ANALYTICS SERVICE - %s] Order recorded on partition %d offset %d. Total orders: %d", as.instanceID, msg.Partition, msg.Offset, atomic.LoadInt64(&as.ordersCount))
			}

			as.consumer.CommitMessage(ctx, msg)
		}
	}
}

func (as *AnalyticsService) GetMetrics() map[string]interface{} {
	return map[string]interface{}{
		"total_orders": atomic.LoadInt64(&as.ordersCount),
	}
}
