// Consumer: đọc từ order-events. Để tái hiện lock/delay rõ ràng, consumer
// này LUÔN coi message là "thiếu context" (giống case OrderCancelled đến
// trước OrderCreated) và escalate theo retry_count:
//
//	retry_count=0 -> publish order-events.retry.10s (đợi 10s)
//	retry_count=1 -> publish order-events.retry.30s (đợi 30s)
//	retry_count=2 -> hết lượt retry -> publish order-events.dlq
//
// Message dead-letter tự quay lại order-events sau khi TTL hết, nhờ cấu
// hình x-dead-letter-exchange/x-dead-letter-routing-key ở internal/mq.
package main

import (
	"log"

	amqp "github.com/rabbitmq/amqp091-go"

	"queue-edge-cases/internal/mq"
)

func retryCountOf(headers amqp.Table) int {
	v, ok := headers[mq.HeaderRetryCount]
	if !ok {
		return 0
	}
	switch n := v.(type) {
	case int32:
		return int(n)
	case int64:
		return int(n)
	case int16:
		return int(n)
	case int:
		return n
	default:
		return 0
	}
}

func main() {
	conn, ch, err := mq.Dial()
	if err != nil {
		log.Fatalf("dial thất bại: %v", err)
	}
	defer conn.Close()
	defer ch.Close()

	if err := ch.Qos(1, 0, false); err != nil {
		log.Fatalf("set qos thất bại: %v", err)
	}

	deliveries, err := ch.Consume(mq.QueueMain, "", false, false, false, false, nil)
	if err != nil {
		log.Fatalf("consume thất bại: %v", err)
	}

	log.Printf("Consumer đang lắng nghe %q...", mq.QueueMain)

	for d := range deliveries {
		eventID, _ := d.Headers[mq.HeaderEventID].(string)
		retryCount := retryCountOf(d.Headers)

		log.Printf("Nhận %s: retry_count=%d, body=%s", eventID, retryCount, string(d.Body))
		log.Printf("  -> giả lập thiếu context, cần retry")

		nextQueue, isDLQ := mq.NextQueueForRetry(retryCount)

		headers := amqp.Table{
			mq.HeaderEventID:    eventID,
			mq.HeaderEventType:  d.Headers[mq.HeaderEventType],
			mq.HeaderRetryCount: int32(retryCount + 1),
		}

		if err := mq.Publish(ch, nextQueue, d.Body, headers); err != nil {
			log.Printf("  publish sang %q thất bại: %v", nextQueue, err)
			_ = d.Nack(false, true)
			continue
		}

		if isDLQ {
			log.Printf("  -> hết lượt retry (retry_count=%d) => publish vào %q để manual check", retryCount, nextQueue)
		} else {
			log.Printf("  -> publish vào %q (retry_count=%d)", nextQueue, retryCount+1)
		}

		if err := d.Ack(false); err != nil {
			log.Printf("  ack thất bại: %v", err)
		}
	}
}
