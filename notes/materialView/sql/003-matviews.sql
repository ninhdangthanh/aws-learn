-- =====================================================================
-- 003-matviews.sql — View thường vs Materialized view + hàm refresh
-- =====================================================================

-- ---------------------------------------------------------------------
-- 1) VIEW THƯỜNG — chỉ là "macro", không lưu gì cả.
--    Mỗi lần SELECT là Postgres chạy lại toàn bộ aggregate bên dưới.
--    Dùng nó làm mốc so sánh tốc độ.
-- ---------------------------------------------------------------------
CREATE VIEW v_daily_revenue AS
SELECT
    (o.created_at AT TIME ZONE 'UTC')::date       AS day,
    count(DISTINCT o.id)                          AS orders,
    sum(oi.qty)                                   AS units,
    sum(oi.qty * oi.unit_price)::numeric(16, 2)   AS revenue
FROM orders o
JOIN order_items oi ON oi.order_id = o.id
WHERE o.status = 'paid'
GROUP BY 1;

-- ---------------------------------------------------------------------
-- 2) MATERIALIZED VIEW — đúng mục đích chính: PRE-AGGREGATE.
--    Kết quả được tính một lần và ghi xuống đĩa như một cái bảng.
--    SELECT sau đó chỉ là đọc bảng, không join, không group by lại.
-- ---------------------------------------------------------------------
CREATE MATERIALIZED VIEW mv_daily_revenue AS
SELECT
    (o.created_at AT TIME ZONE 'UTC')::date       AS day,
    count(DISTINCT o.id)                          AS orders,
    sum(oi.qty)                                   AS units,
    sum(oi.qty * oi.unit_price)::numeric(16, 2)   AS revenue
FROM orders o
JOIN order_items oi ON oi.order_id = o.id
WHERE o.status = 'paid'
GROUP BY 1
WITH DATA;   -- WITH DATA = tính luôn lúc tạo (mặc định).
             -- WITH NO DATA = tạo rỗng, chưa query được cho tới khi REFRESH.

-- BẮT BUỘC nếu muốn REFRESH ... CONCURRENTLY:
-- phải có ít nhất một UNIQUE INDEX phủ toàn bộ dòng.
CREATE UNIQUE INDEX ux_mv_daily_revenue_day ON mv_daily_revenue (day);

-- Matview là bảng thật -> index thêm để query nhanh cũng được.
CREATE INDEX idx_mv_daily_revenue_rev ON mv_daily_revenue (revenue DESC);


-- ---------------------------------------------------------------------
-- 3) Top sản phẩm — aggregate + window function.
--    Window function là thứ index không cứu được, pre-aggregate thắng rõ.
-- ---------------------------------------------------------------------
CREATE MATERIALIZED VIEW mv_product_sales AS
SELECT
    p.id                                          AS product_id,
    p.name                                        AS product_name,
    p.category                                    AS category,
    sum(oi.qty)                                   AS units_sold,
    sum(oi.qty * oi.unit_price)::numeric(16, 2)   AS revenue,
    rank() OVER (
        PARTITION BY p.category
        ORDER BY sum(oi.qty * oi.unit_price) DESC
    )                                             AS rank_in_category
FROM products p
JOIN order_items oi ON oi.product_id = p.id
JOIN orders o       ON o.id = oi.order_id AND o.status = 'paid'
GROUP BY p.id, p.name, p.category;

CREATE UNIQUE INDEX ux_mv_product_sales_id  ON mv_product_sales (product_id);
CREATE INDEX idx_mv_product_sales_cat_rank  ON mv_product_sales (category, rank_in_category);


-- ---------------------------------------------------------------------
-- 4) KPI một dòng cho màn hình dashboard.
--    Matview 1 dòng vẫn cần unique index -> lấy hằng số 1 làm khoá.
-- ---------------------------------------------------------------------
CREATE MATERIALIZED VIEW mv_dashboard_kpi AS
SELECT
    1                                                 AS id,
    count(DISTINCT o.id)                              AS total_paid_orders,
    count(DISTINCT o.customer_id)                     AS total_customers,
    sum(oi.qty * oi.unit_price)::numeric(16, 2)       AS total_revenue,
    (sum(oi.qty * oi.unit_price)
        / NULLIF(count(DISTINCT o.id), 0))::numeric(12, 2) AS avg_order_value,
    max(o.created_at)                                 AS last_order_at
FROM orders o
JOIN order_items oi ON oi.order_id = o.id
WHERE o.status = 'paid';

CREATE UNIQUE INDEX ux_mv_dashboard_kpi_id ON mv_dashboard_kpi (id);


-- ---------------------------------------------------------------------
-- 5) Hàm refresh dùng chung + ghi log thời gian.
--    CONCURRENTLY: KHÔNG khoá reader, nhưng chậm hơn và cần unique index.
--    Không CONCURRENTLY: nhanh hơn, nhưng giữ ACCESS EXCLUSIVE LOCK ->
--    mọi SELECT lên matview đó bị treo cho tới khi refresh xong.
-- ---------------------------------------------------------------------
CREATE OR REPLACE FUNCTION refresh_matview(p_view text, p_concurrently boolean DEFAULT true)
RETURNS numeric
LANGUAGE plpgsql
AS $$
DECLARE
    t0       timestamptz := clock_timestamp();
    elapsed  numeric;
BEGIN
    IF p_concurrently THEN
        EXECUTE format('REFRESH MATERIALIZED VIEW CONCURRENTLY %I', p_view);
    ELSE
        EXECUTE format('REFRESH MATERIALIZED VIEW %I', p_view);
    END IF;

    elapsed := extract(epoch FROM clock_timestamp() - t0) * 1000;

    INSERT INTO matview_refresh_log (view_name, duration_ms, is_concurrent)
    VALUES (p_view, round(elapsed, 2), p_concurrently);

    RETURN round(elapsed, 2);
END;
$$;

CREATE OR REPLACE FUNCTION refresh_all_matviews()
RETURNS text
LANGUAGE plpgsql
AS $$
DECLARE
    v      text;
    ms     numeric;
    total  numeric := 0;
    parts  text[]  := '{}';
BEGIN
    FOREACH v IN ARRAY ARRAY['mv_daily_revenue', 'mv_product_sales', 'mv_dashboard_kpi']
    LOOP
        ms    := refresh_matview(v, true);
        total := total + ms;
        parts := parts || format('%s=%sms', v, ms);
    END LOOP;

    RETURN format('%s | total=%sms', array_to_string(parts, ' '), round(total, 2));
END;
$$;
