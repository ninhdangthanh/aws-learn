package mq

import amqp "github.com/rabbitmq/amqp091-go"

// Topology dùng default exchange ("") để publish thẳng vào queue theo tên
// (routing key == tên queue). Delay được cấu hình ở chính queue retry qua
// x-message-ttl, và khi TTL hết hạn, x-dead-letter-exchange="" +
// x-dead-letter-routing-key đẩy message thẳng về lại queue chính — không
// cần khai báo exchange riêng.
const (
	QueueMain     = "order-events"
	QueueRetry10s = "order-events.retry.10s"
	QueueRetry30s = "order-events.retry.30s"
	QueueDLQ      = "order-events.dlq"

	HeaderRetryCount = "retry_count"
	HeaderEventID    = "event_id"
	HeaderEventType  = "event_type"
)

// RetryQueues liệt kê theo đúng thứ tự escalate: hết lượt cuối thì đi DLQ.
var RetryQueues = []string{QueueRetry10s, QueueRetry30s}

// NextQueueForRetry quyết định message nên đi tiếp vào queue nào dựa trên số
// lần đã retry. retryCount bắt đầu từ 0 (message gốc, chưa từng retry).
func NextQueueForRetry(retryCount int) (queue string, isDLQ bool) {
	if retryCount < len(RetryQueues) {
		return RetryQueues[retryCount], false
	}
	return QueueDLQ, true
}

func declareTopology(ch *amqp.Channel) error {
	// Main queue có DLX trỏ về DLQ: khi consumer nack(requeue=false) một
	// message (hết lượt retry), RabbitMQ tự dead-letter nó sang order-events.dlq
	// và tự ghi x-death (reason=rejected, count, queue gốc) — atomic, không cần
	// consumer publish tay.
	if _, err := ch.QueueDeclare(QueueMain, true, false, false, false, amqp.Table{
		"x-dead-letter-exchange":    "",
		"x-dead-letter-routing-key": QueueDLQ,
	}); err != nil {
		return err
	}

	if _, err := ch.QueueDeclare(QueueRetry10s, true, false, false, false, amqp.Table{
		"x-message-ttl":             int32(10_000),
		"x-dead-letter-exchange":    "",
		"x-dead-letter-routing-key": QueueMain,
	}); err != nil {
		return err
	}

	if _, err := ch.QueueDeclare(QueueRetry30s, true, false, false, false, amqp.Table{
		"x-message-ttl":             int32(30_000),
		"x-dead-letter-exchange":    "",
		"x-dead-letter-routing-key": QueueMain,
	}); err != nil {
		return err
	}

	if _, err := ch.QueueDeclare(QueueDLQ, true, false, false, false, nil); err != nil {
		return err
	}

	return nil
}

// Publish gửi message vào queue chỉ định qua default exchange (routing key
// = tên queue).
func Publish(ch *amqp.Channel, queue string, body []byte, headers amqp.Table) error {
	return ch.Publish("", queue, false, false, amqp.Publishing{
		ContentType:  "application/json",
		DeliveryMode: amqp.Persistent,
		Headers:      headers,
		Body:         body,
	})
}
