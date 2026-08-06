-- =====================================================================
-- 001-schema.sql — Bảng gốc (source of truth) cho demo materialized view
-- Bài toán: hệ thống bán hàng, dashboard cần doanh thu theo ngày /
-- theo sản phẩm. Query aggregate chạy trên hàng trăm nghìn dòng nên chậm.
-- =====================================================================

CREATE TABLE customers (
    id          bigserial PRIMARY KEY,
    name        text        NOT NULL,
    country     text        NOT NULL,
    created_at  timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE products (
    id          bigserial PRIMARY KEY,
    name        text           NOT NULL,
    category    text           NOT NULL,
    price       numeric(12, 2) NOT NULL
);

CREATE TABLE orders (
    id           bigserial PRIMARY KEY,
    customer_id  bigint      NOT NULL REFERENCES customers (id),
    status       text        NOT NULL CHECK (status IN ('paid', 'pending', 'cancelled')),
    created_at   timestamptz NOT NULL
);

CREATE TABLE order_items (
    id          bigserial PRIMARY KEY,
    order_id    bigint         NOT NULL REFERENCES orders (id),
    product_id  bigint         NOT NULL REFERENCES products (id),
    qty         int            NOT NULL,
    unit_price  numeric(12, 2) NOT NULL
);

-- Index "bình thường" mà một app production sẽ có.
-- Chú ý: có index rồi mà query aggregate vẫn chậm — đó chính là lý do
-- materialized view tồn tại. Index giúp *lọc*, không giúp *cộng dồn*.
CREATE INDEX idx_orders_created_at ON orders (created_at);
CREATE INDEX idx_orders_status     ON orders (status);
CREATE INDEX idx_items_order_id    ON order_items (order_id);
CREATE INDEX idx_items_product_id  ON order_items (product_id);

-- Bảng log để biết matview được refresh lần cuối lúc nào và mất bao lâu.
-- Postgres KHÔNG tự lưu thời điểm refresh, muốn biết thì phải tự ghi.
CREATE TABLE matview_refresh_log (
    id           bigserial PRIMARY KEY,
    view_name    text        NOT NULL,
    refreshed_at timestamptz NOT NULL DEFAULT now(),
    duration_ms  numeric(10, 2) NOT NULL,
    is_concurrent boolean    NOT NULL
);

CREATE INDEX idx_refresh_log_view ON matview_refresh_log (view_name, refreshed_at DESC);
