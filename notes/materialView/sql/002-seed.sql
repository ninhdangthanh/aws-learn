-- =====================================================================
-- 002-seed.sql — Dummy data
-- 5.000 khách, 200 sản phẩm, 150.000 đơn trong 90 ngày, ~450.000 dòng item.
-- Đủ lớn để query aggregate mất vài trăm ms — thấy được lợi ích của matview.
-- =====================================================================

SELECT setseed(0.42);   -- cố định random để mọi người seed ra cùng một bộ số

-- ---------- customers ----------
INSERT INTO customers (name, country, created_at)
SELECT
    'Customer ' || g,
    (ARRAY['VN', 'SG', 'TH', 'ID', 'PH', 'MY'])[1 + floor(random() * 6)::int],
    now() - (random() * 365) * INTERVAL '1 day'
FROM generate_series(1, 5000) g;

-- ---------- products ----------
INSERT INTO products (name, category, price)
SELECT
    'Product ' || g,
    (ARRAY['Coffee', 'Tea', 'Bakery', 'Snack', 'Merch'])[1 + floor(random() * 5)::int],
    (20000 + floor(random() * 180000))::numeric(12, 2)
FROM generate_series(1, 200) g;

-- ---------- orders ----------
-- 90 ngày gần nhất. ~88% paid, 7% pending, 5% cancelled.
INSERT INTO orders (customer_id, status, created_at)
SELECT
    1 + floor(random() * 5000)::bigint,
    CASE
        WHEN random() < 0.88 THEN 'paid'
        WHEN random() < 0.95 THEN 'pending'
        ELSE 'cancelled'
    END,
    now()
        - (floor(random() * 90))          * INTERVAL '1 day'
        - (floor(random() * 24))          * INTERVAL '1 hour'
        - (floor(random() * 60))          * INTERVAL '1 minute'
FROM generate_series(1, 150000) g;

-- ---------- order_items ----------
-- Mỗi đơn 1-5 dòng item (dòng đầu luôn có, 4 dòng sau xác suất 55%)
-- => trung bình ~3,2 dòng/đơn, tổng ~480.000 dòng.
INSERT INTO order_items (order_id, product_id, qty, unit_price)
SELECT x.order_id, p.id, x.qty, p.price
FROM (
    SELECT
        o.id                              AS order_id,
        1 + floor(random() * 200)::bigint AS product_id,
        1 + floor(random() * 3)::int      AS qty
    FROM orders o
    CROSS JOIN generate_series(1, 5) AS s
    WHERE s = 1 OR random() < 0.55
) x
JOIN products p ON p.id = x.product_id;

ANALYZE customers;
ANALYZE products;
ANALYZE orders;
ANALYZE order_items;
