// Seed: tạo bảng test và insert ~2 triệu row bằng gofakeit qua COPY protocol
// (pgx.CopyFrom) để mô phỏng một bảng "production-size" cho lock-demos/.
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"strconv"
	"time"

	"github.com/brianvoe/gofakeit/v7"
	"github.com/jackc/pgx/v5"

	"lock-lab/internal/db"
)

const tableName = "lock_test_orders"

var statuses = []string{"created", "paid", "shipped", "cancelled"}

func envInt(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return fallback
}

// fakeRows sinh dữ liệu giả theo kiểu streaming (không giữ cả batch trong
// slice trước), implement interface pgx.CopyFromSource.
type fakeRows struct {
	remaining int
	from      time.Time
	to        time.Time
	current   []any
}

func newFakeRows(n int) *fakeRows {
	return &fakeRows{
		remaining: n,
		from:      time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC),
		to:        time.Now(),
	}
}

func (f *fakeRows) Next() bool {
	if f.remaining <= 0 {
		return false
	}
	f.remaining--
	f.current = []any{
		gofakeit.Name(),
		gofakeit.Email(),
		gofakeit.RandomString(statuses),
		gofakeit.Price(5, 2000),
		gofakeit.DateRange(f.from, f.to),
	}
	return true
}

func (f *fakeRows) Values() ([]any, error) { return f.current, nil }
func (f *fakeRows) Err() error             { return nil }

func createTable(ctx context.Context, conn *pgx.Conn) error {
	if _, err := conn.Exec(ctx, fmt.Sprintf("DROP TABLE IF EXISTS %s", tableName)); err != nil {
		return err
	}
	_, err := conn.Exec(ctx, fmt.Sprintf(`
		CREATE TABLE %s (
			id BIGSERIAL PRIMARY KEY,
			customer_name TEXT NOT NULL,
			email TEXT NOT NULL,
			status TEXT NOT NULL,
			amount NUMERIC(10, 2) NOT NULL,
			created_at TIMESTAMPTZ NOT NULL
		)
	`, tableName))
	return err
}

func main() {
	totalRows := envInt("TOTAL_ROWS", 2_000_000)
	batchSize := envInt("BATCH_SIZE", 100_000)

	ctx := context.Background()
	conn, err := db.Connect(ctx)
	if err != nil {
		log.Fatalf("connect thất bại: %v", err)
	}
	defer conn.Close(ctx)

	log.Printf("Tạo bảng %q...", tableName)
	if err := createTable(ctx, conn); err != nil {
		log.Fatalf("tạo bảng thất bại: %v", err)
	}

	log.Printf("Bắt đầu insert %d rows, batch size %d...", totalRows, batchSize)
	startedAt := time.Now()
	columns := []string{"customer_name", "email", "status", "amount", "created_at"}

	inserted := 0
	for inserted < totalRows {
		size := min(batchSize, totalRows-inserted)

		n, err := conn.CopyFrom(ctx, pgx.Identifier{tableName}, columns, newFakeRows(size))
		if err != nil {
			log.Fatalf("copy thất bại sau %d rows: %v", inserted, err)
		}
		inserted += int(n)

		log.Printf("  %d/%d rows (%.1fs)", inserted, totalRows, time.Since(startedAt).Seconds())
	}

	log.Println("Đang ANALYZE bảng để planner có số liệu thống kê mới...")
	if _, err := conn.Exec(ctx, fmt.Sprintf("ANALYZE %s", tableName)); err != nil {
		log.Fatalf("analyze thất bại: %v", err)
	}

	log.Printf("Xong. Insert %d rows trong %.1fs.", totalRows, time.Since(startedAt).Seconds())
}
