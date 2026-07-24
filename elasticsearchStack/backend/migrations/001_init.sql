-- Phase 5 — schema: products (source of truth) + outbox (transactional sync intent).

CREATE TABLE IF NOT EXISTS products (
  id          BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  name        TEXT        NOT NULL,
  description TEXT        NOT NULL DEFAULT '',
  sku         TEXT        NOT NULL DEFAULT '',
  status      TEXT        NOT NULL DEFAULT 'active',
  category    TEXT        NOT NULL DEFAULT '',
  brand       TEXT        NOT NULL DEFAULT '',
  price       NUMERIC(12,2) NOT NULL DEFAULT 0,
  in_stock    BOOLEAN     NOT NULL DEFAULT true,
  created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Outbox: mọi thay đổi products ghi 1 dòng ở đây TRONG CÙNG transaction.
-- version = updated_at epoch millis -> dùng làm external version cho ES (chống stale write).
CREATE TABLE IF NOT EXISTS outbox (
  id           BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  aggregate    TEXT        NOT NULL,            -- 'product'
  aggregate_id TEXT        NOT NULL,            -- product id (string, dùng làm ES _id)
  op           TEXT        NOT NULL,            -- 'index' | 'delete'
  version      BIGINT      NOT NULL,            -- updated_at epoch millis
  payload      JSONB       NOT NULL,            -- snapshot doc để index ('{}' cho delete)
  created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
  processed_at TIMESTAMPTZ                      -- NULL = chưa xử lý
);

-- Partial index: worker chỉ quét dòng chưa xử lý -> rẻ dù bảng phình to.
CREATE INDEX IF NOT EXISTS outbox_unprocessed_idx
  ON outbox (id) WHERE processed_at IS NULL;

-- Phase 6 — multi-tenant (6.6): mỗi product thuộc 1 tenant.
-- Access filter được backend ÉP từ context đăng nhập (header X-Tenant-ID),
-- KHÔNG bao giờ lấy tenant từ request body -> user tenant A không thấy data tenant B.
-- Idempotent: ADD COLUMN IF NOT EXISTS chạy lại an toàn ở mỗi lần khởi động.
ALTER TABLE products ADD COLUMN IF NOT EXISTS tenant_id TEXT NOT NULL DEFAULT 'default';
CREATE INDEX IF NOT EXISTS products_tenant_idx ON products (tenant_id);
