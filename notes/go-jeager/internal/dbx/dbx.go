// Package dbx tạo pool kết nối Postgres đã gắn sẵn tracing.
//
// Điểm đáng chú ý: chỉ cần một dòng gán cfg.ConnConfig.Tracer là MỌI câu SQL
// chạy qua pool này tự động sinh span, kèm attribute db.statement chứa nguyên
// văn câu lệnh. Không phải bọc tay từng query.
package dbx

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/exaring/otelpgx"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Connect mở pool tới Postgres, có retry cho lúc compose vừa khởi động.
func Connect(ctx context.Context, dsn string) (*pgxpool.Pool, error) {
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, fmt.Errorf("phân tích DSN: %w", err)
	}

	// Một dòng này bật tracing cho toàn bộ query.
	// WithIncludeQueryParameters đưa cả tham số ($1, $2...) vào span attribute —
	// cực tiện khi debug, nhưng KHÔNG bật ở production vì sẽ lộ dữ liệu người dùng.
	cfg.ConnConfig.Tracer = otelpgx.NewTracer(
		otelpgx.WithIncludeQueryParameters(),
	)
	cfg.MaxConns = 10

	var pool *pgxpool.Pool
	for attempt := 1; attempt <= 15; attempt++ {
		pool, err = pgxpool.NewWithConfig(ctx, cfg)
		if err == nil {
			if err = pool.Ping(ctx); err == nil {
				slog.Info("Postgres sẵn sàng")
				return pool, nil
			}
			pool.Close()
		}
		slog.Warn("chưa kết nối được Postgres, thử lại", "lần", attempt, "lỗi", err)
		time.Sleep(2 * time.Second)
	}
	return nil, fmt.Errorf("kết nối Postgres sau 15 lần thử: %w", err)
}
