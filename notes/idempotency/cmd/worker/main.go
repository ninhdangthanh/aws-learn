package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"log"
	"os"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"

	_ "github.com/lib/pq"
)

const consumerName = "send_order_created_notification"

type orderCreatedEvent struct {
	EventID              string `json:"event_id"`
	OrderID              int64  `json:"order_id"`
	SimulateDelaySeconds int    `json:"simulate_delay_seconds"`
}

func main() {
	postgresDSN := getenv("POSTGRES_DSN", "postgres://app:app@localhost:5432/idempotency_lab?sslmode=disable")
	rabbitURL := getenv("RABBITMQ_URL", "amqp://guest:guest@localhost:5672/")

	db, err := sql.Open("postgres", postgresDSN)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	if err := db.Ping(); err != nil {
		log.Fatal(err)
	}

	conn, err := amqp.Dial(rabbitURL)
	if err != nil {
		log.Fatal(err)
	}
	defer conn.Close()

	ch, err := conn.Channel()
	if err != nil {
		log.Fatal(err)
	}
	defer ch.Close()

	// Prefetch 1 giúp demo dễ nhìn: worker xử lý từng message một.
	// Production có thể tăng lên, nhưng càng nhiều concurrency thì idempotency càng quan trọng.
	if err := ch.Qos(1, 0, false); err != nil {
		log.Fatal(err)
	}

	q, err := ch.QueueDeclare("order.created", true, false, false, false, nil)
	if err != nil {
		log.Fatal(err)
	}

	deliveries, err := ch.Consume(q.Name, "", false, false, false, false, nil)
	if err != nil {
		log.Fatal(err)
	}

	log.Println("worker waiting for RabbitMQ messages")
	for msg := range deliveries {
		if err := handleMessage(context.Background(), db, msg.Body); err != nil {
			log.Println("handle message failed:", err)

			// Không ack khi side effect chưa chắc thành công.
			// Requeue=true để RabbitMQ giao lại message, mô phỏng at-least-once delivery.
			_ = msg.Nack(false, true)
			continue
		}

		// Ack chỉ sau khi transaction side effect + dedup record đã commit.
		// Nếu ack trước rồi process crash, message mất nhưng business chưa xử lý xong.
		_ = msg.Ack(false)
	}
}

func handleMessage(ctx context.Context, db *sql.DB, raw []byte) error {
	var event orderCreatedEvent
	if err := json.Unmarshal(raw, &event); err != nil {
		// Poison message nên đi DLQ trong production.
		// Lab này return nil để ack và tránh loop vô hạn vì JSON sai sẽ không tự hết sai.
		log.Println("invalid json, acking poison message:", err)
		return nil
	}
	if event.EventID == "" || event.OrderID == 0 {
		log.Println("invalid event data, acking poison message")
		return nil
	}

	return createNotificationAndMarkEventSucceeded(ctx, db, event)
}

func createNotificationAndMarkEventSucceeded(ctx context.Context, db *sql.DB, event orderCreatedEvent) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	var insertedEventID string
	err = tx.QueryRowContext(ctx, `
		INSERT INTO processed_events (consumer_name, event_id, status)
		VALUES ($1, $2, 'processing')
		ON CONFLICT (consumer_name, event_id) DO NOTHING
		RETURNING event_id
	`, consumerName, event.EventID).Scan(&insertedEventID)

	if errors.Is(err, sql.ErrNoRows) {
		// Event này đã được consumer này xử lý trước đó.
		// Vì dedup record và notification được commit trong cùng transaction,
		// thấy duplicate ở đây nghĩa là side effect tương ứng cũng đã commit.
		log.Printf("duplicate event ignored event_id=%s", event.EventID)
		return tx.Commit()
	}
	if err != nil {
		return err
	}

	if event.SimulateDelaySeconds > 0 {
		// Sleep sau khi insert dedup record nhưng trước khi commit.
		// Nếu có worker khác xử lý duplicate event cùng lúc, PostgreSQL unique constraint
		// sẽ buộc transaction thứ hai chờ transaction đầu tiên quyết định commit/rollback.
		log.Printf("simulate slow consumer inside transaction: sleeping %d seconds", event.SimulateDelaySeconds)
		time.Sleep(time.Duration(event.SimulateDelaySeconds) * time.Second)
	}

	_, err = tx.ExecContext(ctx, `
		INSERT INTO notifications (order_id, event_id, message)
		VALUES ($1, $2, $3)
	`, event.OrderID, event.EventID, "Order created notification")
	if err != nil {
		return err
	}

	_, err = tx.ExecContext(ctx, `
		UPDATE processed_events
		SET status = 'succeeded', updated_at = now()
		WHERE consumer_name = $1 AND event_id = $2
	`, consumerName, event.EventID)
	if err != nil {
		return err
	}

	return tx.Commit()
}

func getenv(key, fallback string) string {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	return value
}
