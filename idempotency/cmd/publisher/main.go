package main

import (
	"context"
	"encoding/json"
	"flag"
	"log"
	"os"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

type orderCreatedEvent struct {
	EventID              string `json:"event_id"`
	OrderID              int64  `json:"order_id"`
	SimulateDelaySeconds int    `json:"simulate_delay_seconds"`
}

func main() {
	eventID := flag.String("event-id", "event-001", "stable event id used as dedup key")
	orderID := flag.Int64("order-id", 1, "order id to include in the event")
	delay := flag.Int("delay", 0, "worker sleep duration, useful to reproduce slow side effect")
	flag.Parse()

	url := getenv("RABBITMQ_URL", "amqp://guest:guest@localhost:5672/")

	conn, err := amqp.Dial(url)
	if err != nil {
		log.Fatal(err)
	}
	defer conn.Close()

	ch, err := conn.Channel()
	if err != nil {
		log.Fatal(err)
	}
	defer ch.Close()

	q, err := ch.QueueDeclare("order.created", true, false, false, false, nil)
	if err != nil {
		log.Fatal(err)
	}

	body, err := json.Marshal(orderCreatedEvent{
		EventID:              *eventID,
		OrderID:              *orderID,
		SimulateDelaySeconds: *delay,
	})
	if err != nil {
		log.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err = ch.PublishWithContext(ctx, "", q.Name, false, false, amqp.Publishing{
		ContentType:  "application/json",
		DeliveryMode: amqp.Persistent,
		Body:         body,
	})
	if err != nil {
		log.Fatal(err)
	}

	log.Printf("published event_id=%s order_id=%d", *eventID, *orderID)
}

func getenv(key, fallback string) string {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	return value
}
