// Consumer: đọc từ order-events. Để tái hiện lock/delay rõ ràng, consumer
// này LUÔN coi message là "thiếu context" (giống case OrderCancelled đến
// trước OrderCreated) nên mọi lần xử lý business đều fail.
//
// Chỉ có DUY NHẤT một retry queue TTL 30s. Message quay vòng qua nó, mỗi
// vòng header retry_tick +1, và consumer dùng chính header đó để biết mình
// đang ở đâu trên thang retry:
//
//	tick=0                 -> message mới, xử lý luôn
//	tick ∈ {1,2,4,10,20,60} -> tới mốc 30s/1m/2m/5m/10m/30m, xử lý lại
//	tick ở giữa hai mốc     -> bỏ qua, đẩy thẳng lại retry queue
//	tick > 60               -> hết lượt, nack(requeue=false) -> DLQ
//
// Đánh đổi có chủ ý: logic milestone nằm ở chính consumer chính, nên một
// message chờ 30 phút sẽ đi qua đây 60 lần và bị bỏ qua 54 lần. Đổi lại
// không cần scheduler process riêng và topology chỉ còn 3 queue.
package main

import (
	"log"

	amqp "github.com/rabbitmq/amqp091-go"

	"queue-edge-cases/internal/mq"
)

// park đẩy message sang retry queue với tick+1 rồi ack bản cũ. Phải publish
// tay chứ không nack được, vì nack chỉ đẩy được về DLX cố định của main
// queue (là DLQ), không route sang retry queue được.
func park(ch *amqp.Channel, d amqp.Delivery, tick int) {
	if err := mq.Publish(ch, mq.QueueRetry, d.Body, mq.WithTick(d.Headers, tick+1)); err != nil {
		log.Printf("  publish sang %q thất bại: %v", mq.QueueRetry, err)
		_ = d.Nack(false, true)
		return
	}
	if err := d.Ack(false); err != nil {
		log.Printf("  ack thất bại: %v", err)
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
		tick := mq.TickOf(d.Headers)

		switch mq.ActionForTick(tick) {
		case mq.ActionDLQ:
			log.Printf("Nhận %s: tick=%d, đã qua mốc cuối => hết lượt retry", eventID, tick)
			log.Printf("  -> nack, dead-letter về %q để manual check", mq.QueueDLQ)
			if err := d.Nack(false, false); err != nil {
				log.Printf("  nack thất bại: %v", err)
			}

		case mq.ActionWait:
			// Tick giữa hai mốc: không đụng vào business, đẩy lại ngay.
			log.Printf("Nhận %s: tick=%d (chưa tới mốc) -> quay lại %q", eventID, tick, mq.QueueRetry)
			park(ch, d, tick)

		case mq.ActionProcess:
			log.Printf("Nhận %s: tick=%d (đã chờ %v), xử lý: %s",
				eventID, tick, mq.ElapsedForTick(tick), string(d.Body))
			log.Printf("  -> giả lập thiếu context, cần retry")
			log.Printf("  -> park vào %q, mốc kế tiếp sẽ xử lý lại", mq.QueueRetry)
			park(ch, d, tick)
		}
	}
}
