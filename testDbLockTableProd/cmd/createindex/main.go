// CREATE INDEX thường (không CONCURRENTLY): lấy SHARE lock, chặn
// INSERT/UPDATE/DELETE cho tới khi build xong index.
// Chạy song song với ./cmd/longtx để thấy nó phải chờ.
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"lock-lab/internal/db"
)

func main() {
	lockTimeout := os.Getenv("LOCK_TIMEOUT")
	if lockTimeout == "" {
		lockTimeout = "10s"
	}

	ctx := context.Background()
	conn, err := db.Connect(ctx)
	if err != nil {
		log.Fatalf("connect thất bại: %v", err)
	}
	defer conn.Close(ctx)

	if _, err := conn.Exec(ctx, fmt.Sprintf("SET lock_timeout = '%s'", lockTimeout)); err != nil {
		log.Fatalf("set lock_timeout thất bại: %v", err)
	}
	if _, err := conn.Exec(ctx, "DROP INDEX IF EXISTS idx_lock_test_orders_email"); err != nil {
		log.Fatalf("drop index thất bại: %v", err)
	}

	log.Printf("CREATE INDEX (blocking)... (lock_timeout=%s)", lockTimeout)
	startedAt := time.Now()

	_, err = conn.Exec(ctx, "CREATE INDEX idx_lock_test_orders_email ON lock_test_orders(email)")
	if err != nil {
		log.Fatalf("thất bại sau %.1fs: %v", time.Since(startedAt).Seconds(), err)
	}

	log.Printf("Xong sau %.1fs.", time.Since(startedAt).Seconds())
}
