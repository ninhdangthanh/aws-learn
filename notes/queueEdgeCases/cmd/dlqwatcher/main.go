// DLQ watcher: mô phỏng bước "manual check" trên production — không tự
// động xử lý lại, chỉ in ra đầy đủ thông tin để người vận hành soi và
// quyết định (bỏ, replay tay, hay báo bug). Ack sau khi đã "xem" xong.
package main

import (
	"log"

	"queue-edge-cases/internal/mq"
)

func main() {
	conn, ch, err := mq.Dial()
	if err != nil {
		log.Fatalf("dial thất bại: %v", err)
	}
	defer conn.Close()
	defer ch.Close()

	deliveries, err := ch.Consume(mq.QueueDLQ, "", false, false, false, false, nil)
	if err != nil {
		log.Fatalf("consume thất bại: %v", err)
	}

	log.Printf("DLQ watcher đang lắng nghe %q. Ctrl+C để dừng.", mq.QueueDLQ)

	for d := range deliveries {
		log.Println("=== MESSAGE VÀO DLQ, CẦN MANUAL CHECK ===")
		log.Printf("headers: %+v", d.Headers)
		log.Printf("body:    %s", string(d.Body))
		log.Println("==========================================")

		if err := d.Ack(false); err != nil {
			log.Printf("ack thất bại: %v", err)
		}
	}
}
