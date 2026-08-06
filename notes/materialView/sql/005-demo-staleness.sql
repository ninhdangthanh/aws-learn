-- =====================================================================
-- DEMO 2 — Cái giá phải trả: DỮ LIỆU CŨ (stale)
-- Matview là ảnh chụp tại thời điểm refresh. Ghi mới vào bảng gốc
-- KHÔNG tự động chảy vào matview.
-- =====================================================================

\echo '--- Trước khi thêm đơn ---'
SELECT
    (SELECT revenue FROM mv_daily_revenue
      WHERE day = (now() AT TIME ZONE 'UTC')::date)              AS revenue_matview,
    (SELECT revenue FROM v_daily_revenue
      WHERE day = (now() AT TIME ZONE 'UTC')::date)              AS revenue_realtime;

\echo ''
\echo '--- Thêm 1 đơn 999.999.999đ vào hôm nay ---'
WITH o AS (
    INSERT INTO orders (customer_id, status, created_at)
    VALUES (1, 'paid', now())
    RETURNING id
)
INSERT INTO order_items (order_id, product_id, qty, unit_price)
SELECT o.id, 1, 1, 999999999 FROM o;

\echo ''
\echo '--- Ngay sau khi thêm: matview VẪN CŨ, view thường đã thấy ---'
SELECT
    (SELECT revenue FROM mv_daily_revenue
      WHERE day = (now() AT TIME ZONE 'UTC')::date)              AS revenue_matview,
    (SELECT revenue FROM v_daily_revenue
      WHERE day = (now() AT TIME ZONE 'UTC')::date)              AS revenue_realtime;

\echo ''
\echo '--- Refresh tay ---'
SELECT refresh_matview('mv_daily_revenue') AS took_ms;

\echo ''
\echo '--- Sau refresh: hai con số khớp nhau ---'
SELECT
    (SELECT revenue FROM mv_daily_revenue
      WHERE day = (now() AT TIME ZONE 'UTC')::date)              AS revenue_matview,
    (SELECT revenue FROM v_daily_revenue
      WHERE day = (now() AT TIME ZONE 'UTC')::date)              AS revenue_realtime;

\echo ''
\echo '=> Với refresh 30s, dashboard trễ tối đa 30s. Chấp nhận được với'
\echo '   báo cáo/BI. KHÔNG chấp nhận được với số dư ví, tồn kho, giá bán.'
