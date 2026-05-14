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
	groupID     string
	ordersCount int64
	ordersValue float64
}

func NewAnalyticsService(brokers []string, instanceID string) *AnalyticsService {
	groupID := "analytics-service"
	consumer := kafka.NewConsumer(brokers, "orders", groupID, 0)

	// Register with the consumer registry
	kafka.GetRegistry().Register(groupID, instanceID)

	return &AnalyticsService{
		consumer:   consumer,
		instanceID: instanceID,
		groupID:    groupID,
	}
}

func (as *AnalyticsService) Start(ctx context.Context) {
	log.Printf("[ANALYTICS SERVICE - %s] started listening to 'orders' topic", as.instanceID)

	defer func() {
		as.consumer.Close()
		kafka.GetRegistry().Unregister(as.groupID, as.instanceID)
		log.Printf("[ANALYTICS SERVICE - %s] stopped", as.instanceID)
	}()

	for {
		select {
		case <-ctx.Done():
			return
		default:
			msg, err := as.consumer.ReadMessage(ctx)
			if err != nil {
				if ctx.Err() != nil {
					return
				}
				log.Printf("[ANALYTICS SERVICE - %s] Error reading message: %v", as.instanceID, err)
				continue
			}

			// Track partition assignment and message processing
			kafka.GetRegistry().RecordMessage(as.groupID, as.instanceID, msg.Partition)

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
