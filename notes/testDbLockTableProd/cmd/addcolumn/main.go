// ALTER TABLE ... ADD COLUMN ... NOT NULL DEFAULT ...: lấy ACCESS EXCLUSIVE
// lock — chặn cả read lẫn write trong lúc chờ tới lượt.
// Chạy song song với ./cmd/longtx để thấy nó xếp hàng phía sau.
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
	if _, err := conn.Exec(ctx, "ALTER TABLE lock_test_orders DROP COLUMN IF EXISTS is_priority"); err != nil {
		log.Fatalf("drop column thất bại: %v", err)
	}

	log.Printf("ALTER TABLE ADD COLUMN NOT NULL DEFAULT... (lock_timeout=%s)", lockTimeout)
	startedAt := time.Now()

	_, err = conn.Exec(ctx, "ALTER TABLE lock_test_orders ADD COLUMN is_priority boolean NOT NULL DEFAULT false")
	if err != nil {
		log.Fatalf("thất bại sau %.1fs: %v", time.Since(startedAt).Seconds(), err)
	}

	log.Printf("Xong sau %.1fs.", time.Since(startedAt).Seconds())
}
