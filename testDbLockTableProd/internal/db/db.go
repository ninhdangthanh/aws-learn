// Package db chứa cấu hình kết nối Postgres dùng chung cho seeder và các
// lock-demos, đọc từ env var để khớp với docker-compose.yml.
package db

import (
	"context"
	"fmt"
	"os"

	"github.com/jackc/pgx/v5"
)

func dsn() string {
	host := getEnv("PGHOST", "localhost")
	port := getEnv("PGPORT", "5432")
	user := getEnv("PGUSER", "app")
	password := getEnv("PGPASSWORD", "app")
	database := getEnv("PGDATABASE", "lock_lab")

	return fmt.Sprintf("postgres://%s:%s@%s:%s/%s", user, password, host, port, database)
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// Connect mở một connection đơn (không dùng pool) — phù hợp với các
// lock-demos vì mỗi session cần giữ đúng 1 connection để BEGIN/COMMIT
// hoặc chạy CREATE INDEX CONCURRENTLY (không được nằm trong transaction block).
func Connect(ctx context.Context) (*pgx.Conn, error) {
	return pgx.Connect(ctx, dsn())
}
