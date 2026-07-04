// Mô phỏng "session A" giữ transaction mở lâu trên production, giống một
// request chậm hoặc job quên COMMIT. Chạy trước, rồi mở terminal khác chạy
// createindex / addcolumn để xem chúng bị block ra sao.
package main

import (
	"context"
	"log"
	"os"
	"strconv"
	"time"

	"lock-lab/internal/db"
)

func main() {
	holdSeconds := 30
	if v := os.Getenv("HOLD_SECONDS"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			holdSeconds = n
		}
	}

	ctx := context.Background()
	conn, err := db.Connect(ctx)
	if err != nil {
		log.Fatalf("connect thất bại: %v", err)
	}
	defer conn.Close(ctx)

	log.Println("BEGIN transaction...")
	tx, err := conn.Begin(ctx)
	if err != nil {
		log.Fatalf("begin thất bại: %v", err)
	}

	log.Println("UPDATE 1 row (giữ ROW EXCLUSIVE lock trên table + row lock)...")
	_, err = tx.Exec(ctx,
		`UPDATE lock_test_orders SET status = 'paid' WHERE id = (SELECT id FROM lock_test_orders LIMIT 1)`)
	if err != nil {
		log.Fatalf("update thất bại: %v", err)
	}

	log.Printf("Giữ transaction mở trong %ds. Mở terminal khác và thử:", holdSeconds)
	log.Println("  go run ./cmd/createindex")
	log.Println("  go run ./cmd/addcolumn")
	log.Println("để xem chúng phải xếp hàng chờ lock...")

	time.Sleep(time.Duration(holdSeconds) * time.Second)

	log.Println("COMMIT. Giải phóng lock.")
	if err := tx.Commit(ctx); err != nil {
		log.Fatalf("commit thất bại: %v", err)
	}
}
