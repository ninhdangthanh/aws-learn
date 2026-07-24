package store

import (
	"context"
)

// OutboxRow — 1 ý định sync (index/delete) chờ worker xử lý.
type OutboxRow struct {
	ID          int64
	AggregateID string // = product id (ES _id)
	Op          string // "index" | "delete"
	Version     int64  // external version cho ES
	Payload     []byte // snapshot doc ('{}' cho delete)
}

// ProcessOutboxBatch — lấy tối đa `limit` dòng chưa xử lý bằng
// FOR UPDATE SKIP LOCKED (nhiều worker song song an toàn), gọi handle() cho từng dòng;
// dòng nào handle() trả nil thì đánh dấu processed_at trong CÙNG tx.
// ES fail -> handle() trả lỗi -> KHÔNG mark -> vòng sau retry (at-least-once).
func (s *Store) ProcessOutboxBatch(ctx context.Context, limit int, handle func(OutboxRow) error) (int, error) {
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback(ctx)

	rows, err := tx.Query(ctx, `
		SELECT id, aggregate_id, op, version, payload
		FROM outbox
		WHERE processed_at IS NULL
		ORDER BY id
		LIMIT $1
		FOR UPDATE SKIP LOCKED`, limit)
	if err != nil {
		return 0, err
	}
	var batch []OutboxRow
	for rows.Next() {
		var r OutboxRow
		if err := rows.Scan(&r.ID, &r.AggregateID, &r.Op, &r.Version, &r.Payload); err != nil {
			rows.Close()
			return 0, err
		}
		batch = append(batch, r)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return 0, err
	}

	done := 0
	for _, r := range batch {
		if err := handle(r); err != nil {
			continue // để lại retry vòng sau, không chặn các dòng khác
		}
		if _, err := tx.Exec(ctx, `UPDATE outbox SET processed_at = now() WHERE id=$1`, r.ID); err != nil {
			return done, err
		}
		done++
	}
	if err := tx.Commit(ctx); err != nil {
		return done, err
	}
	return done, nil
}

// ListProducts — danh sách product mới nhất từ Postgres (source of truth, cho Admin UI).
// tenantID ép cùng access context với Search UI.
func (s *Store) ListProducts(ctx context.Context, tenantID string, limit int) ([]Product, error) {
	if tenantID == "" {
		tenantID = "default"
	}
	rows, err := s.Pool.Query(ctx, `SELECT `+cols+` FROM products WHERE tenant_id=$1 ORDER BY id DESC LIMIT $2`, tenantID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Product
	for rows.Next() {
		p, err := scanProduct(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// PendingOutbox — số dòng chưa xử lý (để test/quan sát).
func (s *Store) PendingOutbox(ctx context.Context) (int64, error) {
	var n int64
	err := s.Pool.QueryRow(ctx, `SELECT count(*) FROM outbox WHERE processed_at IS NULL`).Scan(&n)
	return n, err
}

// CountProducts — COUNT(*) products (đối soát với _count của ES).
func (s *Store) CountProducts(ctx context.Context) (int64, error) {
	var n int64
	err := s.Pool.QueryRow(ctx, `SELECT count(*) FROM products`).Scan(&n)
	return n, err
}

// ScanProductsAfter — keyset pagination theo id (dùng cho backfill, idempotent).
// Trả tối đa `limit` product có id > afterID.
func (s *Store) ScanProductsAfter(ctx context.Context, afterID int64, limit int) ([]Product, error) {
	rows, err := s.Pool.Query(ctx,
		`SELECT `+cols+` FROM products WHERE id > $1 ORDER BY id LIMIT $2`, afterID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Product
	for rows.Next() {
		p, err := scanProduct(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}
