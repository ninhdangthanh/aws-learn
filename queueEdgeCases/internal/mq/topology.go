package mq

import (
	"maps"
	"slices"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

// Topology dùng default exchange ("") để publish thẳng vào queue theo tên
// (routing key == tên queue). Chỉ có DUY NHẤT một retry queue với TTL cố
// định 30s; các mốc delay dài hơn được tạo ra bằng cách cho message quay
// vòng qua queue đó nhiều lần, không phải bằng cách khai báo thêm queue.
//
// Cố định TTL cho mọi message trong queue là điều kiện bắt buộc: RabbitMQ
// chỉ kiểm tra hết hạn ở ĐẦU queue, nên nếu dùng per-message TTL thì một
// message delay 30 phút nằm đầu sẽ chặn message delay 30s nằm sau nó
// (head-of-line blocking). Cùng TTL thì thứ tự hết hạn trùng thứ tự vào
// queue, không bao giờ kẹt.
const (
	QueueMain  = "order-events"
	QueueRetry = "order-events.retry"
	QueueDLQ   = "order-events.dlq"

	// RetryTick là TTL của retry queue, cũng là đơn vị đo của RetryTickMarks.
	RetryTick = 30 * time.Second

	HeaderTick      = "retry_tick"
	HeaderEventID   = "event_id"
	HeaderEventType = "event_type"
)

// RetryTickMarks là các MỐC TÍCH LUỸ kể từ lần fail đầu tiên, tính bằng số
// tick 30s, mà tại đó consumer được phép xử lý lại:
//
//	1 -> 30s, 2 -> 1m, 4 -> 2m, 10 -> 5m, 20 -> 10m, 60 -> 30m
//
// Message vẫn quay vòng qua retry queue ở mọi tick ở giữa (3, 5, 6, 7...),
// consumer chỉ nhìn header rồi đẩy nó trở lại chứ không xử lý business.
// Tổng vòng đời tối đa = mốc cuối = 60 tick = 30 phút, đổi lại 60 lượt
// hop qua broker nhưng chỉ 6 lần xử lý thật.
var RetryTickMarks = []int{1, 2, 4, 10, 20, 60}

// TickAction là quyết định cho một message dựa trên số tick nó đã đi qua.
type TickAction int

const (
	// ActionProcess: message mới (tick=0) hoặc đã tới đúng mốc -> xử lý business.
	ActionProcess TickAction = iota
	// ActionWait: đang ở tick giữa hai mốc -> đẩy lại retry queue, tick+1.
	ActionWait
	// ActionDLQ: đã vượt mốc cuối -> hết lượt retry.
	ActionDLQ
)

// ActionForTick quyết định phải làm gì với message đã đi qua tick vòng retry.
// tick=0 nghĩa là message vừa từ producer vào, chưa từng retry.
func ActionForTick(tick int) TickAction {
	if tick > RetryTickMarks[len(RetryTickMarks)-1] {
		return ActionDLQ
	}
	if tick == 0 || slices.Contains(RetryTickMarks, tick) {
		return ActionProcess
	}
	return ActionWait
}

// ElapsedForTick là thời gian đã trôi qua kể từ lần fail đầu tiên, dùng để log.
func ElapsedForTick(tick int) time.Duration {
	return time.Duration(tick) * RetryTick
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

	// Retry queue KHÔNG có consumer. Message nằm đây đúng 30s rồi TTL hết,
	// DLX đẩy nó về lại main queue để consumer đọc header và quyết định tiếp.
	if _, err := ch.QueueDeclare(QueueRetry, true, false, false, false, amqp.Table{
		"x-message-ttl":             int32(RetryTick.Milliseconds()),
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

// TickOf đọc số tick từ header. AMQP có thể trả về nhiều kiểu integer khác
// nhau tuỳ client đã publish, nên phải switch hết.
func TickOf(headers amqp.Table) int {
	v, ok := headers[HeaderTick]
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

// WithTick copy toàn bộ header gốc rồi ghi đè số tick, để các header do
// producer đặt (trace id, tenant...) không bị mất qua mỗi vòng retry.
func WithTick(headers amqp.Table, tick int) amqp.Table {
	next := amqp.Table{}
	maps.Copy(next, headers)
	next[HeaderTick] = int32(tick)
	return next
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
