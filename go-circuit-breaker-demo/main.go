// Demo circuit breaker (sony/gobreaker) trong hệ thống thương mại điện tử.
//
// Kịch bản: order service gọi payment gateway. Gateway sập, rồi phục hồi.
// Ta quan sát breaker đi qua đủ 3 trạng thái Closed -> Open -> Half-open -> Closed.
package main

import (
	"log"
	"time"

	"gobreaker-demo/internal/order"
	"gobreaker-demo/internal/payment"
)

func main() {
	log.SetFlags(0)

	gw := payment.NewGateway()
	svc := order.NewService(gw)

	step := func(title string) {
		log.Printf("\n=== %s ===", title)
	}

	// ---------------------------------------------------------------
	step("1. Gateway khoẻ — breaker Closed, mọi đơn đi thẳng")
	svc.Place("order-1", 100)
	svc.Place("order-2", 250)
	log.Printf("  %s", svc.Summary())

	// ---------------------------------------------------------------
	step("2. Gateway sập — 3 lỗi liên tiếp làm breaker trip")
	gw.SetHealthy(false)
	svc.Place("order-3", 100)
	svc.Place("order-4", 100)
	svc.Place("order-5", 100) // lần thứ 3 -> ReadyToTrip -> Open
	log.Printf("  %s", svc.Summary())

	// ---------------------------------------------------------------
	step("3. Breaker Open — fail-fast, gateway không bị gọi thêm lần nào")
	before := gw.Calls()
	for _, id := range []string{"order-6", "order-7", "order-8", "order-9"} {
		svc.Place(id, 100)
	}
	log.Printf("  gateway calls: %d -> %d (đứng yên = fail-fast hoạt động)", before, gw.Calls())
	log.Printf("  %s", svc.Summary())

	// ---------------------------------------------------------------
	step("4. Hết Timeout -> Half-open, nhưng gateway vẫn chết -> Open lại")
	time.Sleep(2100 * time.Millisecond)
	svc.Place("order-10", 100) // request thăm dò, thất bại -> quay lại Open
	log.Printf("  %s", svc.Summary())

	// ---------------------------------------------------------------
	step("5. Gateway phục hồi -> request thăm dò thành công -> Closed")
	gw.SetHealthy(true)
	time.Sleep(2100 * time.Millisecond)
	svc.Place("order-11", 100) // thăm dò OK -> MaxRequests=1 đạt -> Closed
	log.Printf("  %s", svc.Summary())

	// ---------------------------------------------------------------
	step("6. Trở lại bình thường")
	svc.Place("order-12", 500)
	log.Printf("  %s", svc.Summary())

	log.Printf("\nĐơn cần retry sau: %v", svc.Queued())
}
