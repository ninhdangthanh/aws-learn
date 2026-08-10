// Producer: publish 1 message test vào order-events với retry_tick=0.
// Dùng để châm ngòi cho thang retry 30s/1m/2m/5m/10m/30m -> DLQ ở consumer.
package main

import (
	"encoding/json"
	"log"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"

	"queue-edge-cases/internal/mq"
)

type orderEvent struct {
	EventID   string `json:"event_id"`
	OrderID   string `json:"order_id"`
	EventType string `json:"event_type"`
}

func main() {
	conn, ch, err := mq.Dial()
	if err != nil {
		log.Fatalf("dial thất bại: %v", err)
	}
	defer conn.Close()
	defer ch.Close()

	event := orderEvent{
		EventID:   time.Now().Format("evt_20060102150405.000"),
		OrderID:   "123",
		EventType: "OrderCancelled",
	}

	body, err := json.Marshal(event)
	if err != nil {
		log.Fatalf("marshal thất bại: %v", err)
	}

	headers := amqp.Table{
		mq.HeaderEventID:   event.EventID,
		mq.HeaderEventType: event.EventType,
		mq.HeaderTick:      int32(0),
	}

	if err := mq.Publish(ch, mq.QueueMain, body, headers); err != nil {
		log.Fatalf("publish thất bại: %v", err)
	}

	log.Printf("Đã publish %s vào %q (retry_tick=0)", event.EventID, mq.QueueMain)
}
