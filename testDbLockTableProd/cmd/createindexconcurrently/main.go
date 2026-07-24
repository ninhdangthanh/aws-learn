// CREATE INDEX CONCURRENTLY: lấy SHARE UPDATE EXCLUSIVE lock, không chặn
// INSERT/UPDATE/DELETE bình thường. Chạy song song với ./cmd/longtx để so
// sánh với ./cmd/createindex (bản blocking).
//
// Lưu ý: không được chạy trong transaction block, nên mỗi conn.Exec ở đây
// tự chạy ngoài transaction (không có Begin/Commit).
package main

import (
	"context"
	"log"
	"time"

	"lock-lab/internal/db"
)

func main() {
	ctx := context.Background()
	conn, err := db.Connect(ctx)
	if err != nil {
		log.Fatalf("connect thất bại: %v", err)
	}
	defer conn.Close(ctx)

	if _, err := conn.Exec(ctx, "DROP INDEX CONCURRENTLY IF EXISTS idx_lock_test_orders_email"); err != nil {
		log.Fatalf("drop index thất bại: %v", err)
	}

	log.Println("CREATE INDEX CONCURRENTLY...")
	startedAt := time.Now()

	_, err = conn.Exec(ctx, "CREATE INDEX CONCURRENTLY idx_lock_test_orders_email ON lock_test_orders(email)")
	if err != nil {
		log.Printf("thất bại sau %.1fs: %v", time.Since(startedAt).Seconds(), err)
		log.Fatal("nếu fail giữa chừng, index có thể ở trạng thái INVALID, cần DROP rồi tạo lại.")
	}

	log.Printf("Xong sau %.1fs (không block write).", time.Since(startedAt).Seconds())
}
