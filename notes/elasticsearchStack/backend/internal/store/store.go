// Package store — Postgres là SOURCE OF TRUTH cho products.
// Mọi thay đổi ghi products + outbox TRONG CÙNG transaction (khi dùng outbox).
package store

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Store struct{ Pool *pgxpool.Pool }

func New(ctx context.Context, dsn string) (*Store, error) {
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return nil, err
	}
	if err := pool.Ping(ctx); err != nil {
		return nil, err
	}
	return &Store{Pool: pool}, nil
}

func (s *Store) Close() { s.Pool.Close() }

// Product — vừa là row Postgres vừa là document ES (_source). id = ES _id.
type Product struct {
	ID          int64     `json:"id"`
	TenantID    string    `json:"tenant_id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	SKU         string    `json:"sku"`
	Status      string    `json:"status"`
	Category    string    `json:"category"`
	Brand       string    `json:"brand"`
	Price       float64   `json:"price"`
	InStock     bool      `json:"in_stock"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// Version — external version cho ES = updated_at epoch millis (đơn điệu tăng theo thời gian sửa).
func (p Product) Version() int64 { return p.UpdatedAt.UnixMilli() }

func (p Product) doc() []byte { b, _ := json.Marshal(p); return b }

// Migrate — chạy DDL (idempotent).
func (s *Store) Migrate(ctx context.Context, ddl string) error {
	_, err := s.Pool.Exec(ctx, ddl)
	return err
}

const cols = `id, tenant_id, name, description, sku, status, category, brand, price, in_stock, created_at, updated_at`

func scanProduct(row pgx.Row) (Product, error) {
	var p Product
	err := row.Scan(&p.ID, &p.TenantID, &p.Name, &p.Description, &p.SKU, &p.Status, &p.Category,
		&p.Brand, &p.Price, &p.InStock, &p.CreatedAt, &p.UpdatedAt)
	return p, err
}

// CreateProduct — INSERT product; nếu withOutbox thì ghi outbox trong CÙNG tx.
func (s *Store) CreateProduct(ctx context.Context, p Product, withOutbox bool) (Product, error) {
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return Product{}, err
	}
	defer tx.Rollback(ctx)

	// tenant_id do handler stamp từ context (không lấy từ body). Default 'default' nếu rỗng.
	if p.TenantID == "" {
		p.TenantID = "default"
	}
	row := tx.QueryRow(ctx, `
		INSERT INTO products (tenant_id, name, description, sku, status, category, brand, price, in_stock)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)
		RETURNING `+cols,
		p.TenantID, p.Name, p.Description, p.SKU, p.Status, p.Category, p.Brand, p.Price, p.InStock)
	created, err := scanProduct(row)
	if err != nil {
		return Product{}, err
	}
	if withOutbox {
		if err := insertOutbox(ctx, tx, created, "index"); err != nil {
			return Product{}, err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return Product{}, err
	}
	return created, nil
}

// UpdateProduct — cập nhật (bump updated_at) + outbox trong cùng tx.
// tenantID ép access boundary: tenant này không sửa được product tenant khác.
func (s *Store) UpdateProduct(ctx context.Context, tenantID string, p Product, withOutbox bool) (Product, error) {
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return Product{}, err
	}
	defer tx.Rollback(ctx)
	if tenantID == "" {
		tenantID = "default"
	}

	row := tx.QueryRow(ctx, `
		UPDATE products
		SET name=$2, description=$3, sku=$4, status=$5, category=$6, brand=$7,
		    price=$8, in_stock=$9, updated_at=now()
		WHERE id=$1 AND tenant_id=$10
		RETURNING `+cols,
		p.ID, p.Name, p.Description, p.SKU, p.Status, p.Category, p.Brand, p.Price, p.InStock, tenantID)
	updated, err := scanProduct(row)
	if err != nil {
		return Product{}, err // pgx.ErrNoRows nếu id không tồn tại
	}
	if withOutbox {
		if err := insertOutbox(ctx, tx, updated, "index"); err != nil {
			return Product{}, err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return Product{}, err
	}
	return updated, nil
}

// DeleteProduct — xóa row + outbox op=delete (version = thời điểm xóa) trong cùng tx.
// tenantID ép access boundary: tenant này không xóa được product tenant khác.
func (s *Store) DeleteProduct(ctx context.Context, tenantID string, id int64, withOutbox bool) error {
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if tenantID == "" {
		tenantID = "default"
	}

	ct, err := tx.Exec(ctx, `DELETE FROM products WHERE id=$1 AND tenant_id=$2`, id, tenantID)
	if err != nil {
		return err
	}
	if ct.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	if withOutbox {
		tomb := Product{ID: id, UpdatedAt: time.Now()}
		if _, err := tx.Exec(ctx, `
			INSERT INTO outbox (aggregate, aggregate_id, op, version, payload)
			VALUES ('product', $1, 'delete', $2, '{}'::jsonb)`,
			fmt.Sprint(id), tomb.Version()); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

// GetProduct — đọc 1 product từ SQL (không phải search).
func (s *Store) GetProduct(ctx context.Context, id int64) (Product, error) {
	return scanProduct(s.Pool.QueryRow(ctx, `SELECT `+cols+` FROM products WHERE id=$1`, id))
}

func insertOutbox(ctx context.Context, tx pgx.Tx, p Product, op string) error {
	_, err := tx.Exec(ctx, `
		INSERT INTO outbox (aggregate, aggregate_id, op, version, payload)
		VALUES ('product', $1, $2, $3, $4::jsonb)`,
		fmt.Sprint(p.ID), op, p.Version(), string(p.doc()))
	return err
}
