-- Chạy tự động một lần khi container postgres khởi tạo volume lần đầu.
-- Muốn nạp lại từ đầu: make reset (xoá volume rồi dựng lại).

CREATE TABLE IF NOT EXISTS orders (
    id         uuid PRIMARY KEY,
    sku        text        NOT NULL,
    qty        integer     NOT NULL CHECK (qty > 0),
    customer   text        NOT NULL,
    status     text        NOT NULL DEFAULT 'PENDING',
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS orders_status_idx ON orders (status);

CREATE TABLE IF NOT EXISTS inventory (
    sku        text PRIMARY KEY,
    name       text        NOT NULL,
    qty        integer     NOT NULL CHECK (qty >= 0),
    updated_at timestamptz NOT NULL DEFAULT now()
);

-- Tồn kho ban đầu.
-- SKU-RARE cố tình để 2 cái: đặt qty=5 là hết hàng thật, không cần fail mode.
INSERT INTO inventory (sku, name, qty) VALUES
    ('SKU-LAPTOP', 'Laptop 14 inch',  50),
    ('SKU-PHONE',  'Điện thoại',     100),
    ('SKU-MOUSE',  'Chuột không dây', 30),
    ('SKU-RARE',   'Hàng hiếm',        2)
ON CONFLICT (sku) DO NOTHING;
