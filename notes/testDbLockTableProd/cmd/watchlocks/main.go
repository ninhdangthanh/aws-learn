// Poll pg_locks + pg_stat_activity mỗi giây để xem ai đang block ai.
// Chạy song song 2-3 lock-demos khác để quan sát blocking chain theo thời
// gian thực.
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"strconv"
	"time"

	"lock-lab/internal/db"
)

const query = `
	SELECT
		blocked.pid AS blocked_pid,
		blocked.query AS blocked_query,
		blocking.pid AS blocking_pid,
		blocking.query AS blocking_query
	FROM pg_locks bl
	JOIN pg_stat_activity blocked ON blocked.pid = bl.pid
	JOIN pg_locks kl ON kl.locktype = bl.locktype
		AND kl.database IS NOT DISTINCT FROM bl.database
		AND kl.relation IS NOT DISTINCT FROM bl.relation
		AND kl.pid != bl.pid
		AND kl.granted
	JOIN pg_stat_activity blocking ON blocking.pid = kl.pid
	WHERE NOT bl.granted
`

func main() {
	intervalMs := 1000
	if v := os.Getenv("INTERVAL_MS"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			intervalMs = n
		}
	}

	ctx := context.Background()
	conn, err := db.Connect(ctx)
	if err != nil {
		log.Fatalf("connect thất bại: %v", err)
	}
	defer conn.Close(ctx)

	fmt.Printf("Theo dõi lock mỗi %dms. Ctrl+C để dừng.\n", intervalMs)

	ticker := time.NewTicker(time.Duration(intervalMs) * time.Millisecond)
	defer ticker.Stop()

	for range ticker.C {
		rows, err := conn.Query(ctx, query)
		if err != nil {
			log.Printf("lỗi khi query pg_locks: %v", err)
			continue
		}

		fmt.Print("\033[H\033[2J")
		fmt.Println(time.Now().Format(time.RFC3339))

		found := false
		for rows.Next() {
			found = true
			var blockedPID, blockingPID int32
			var blockedQuery, blockingQuery string
			if err := rows.Scan(&blockedPID, &blockedQuery, &blockingPID, &blockingQuery); err != nil {
				log.Printf("scan lỗi: %v", err)
				continue
			}
			fmt.Printf("PID %d bị chặn bởi PID %d\n", blockedPID, blockingPID)
			fmt.Printf("  blocked:  %s\n", blockedQuery)
			fmt.Printf("  blocking: %s\n", blockingQuery)
		}
		rows.Close()

		if !found {
			fmt.Println("(không có session nào đang bị block)")
		}
	}
}
