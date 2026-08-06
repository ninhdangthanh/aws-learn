-- =====================================================================
-- DEMO 1 — Mục đích chính: PRE-AGGREGATE
-- Cùng một kết quả, một bên tính lại từ đầu, một bên đọc sẵn.
-- =====================================================================

\timing on

\echo '--- (a) VIEW thường: join 450k dòng + group by mỗi lần gọi ---'
EXPLAIN (ANALYZE, BUFFERS, COSTS OFF, TIMING OFF)
SELECT * FROM v_daily_revenue ORDER BY day DESC LIMIT 7;

\echo ''
\echo '--- (b) MATERIALIZED VIEW: index scan trên bảng đã tính sẵn ---'
EXPLAIN (ANALYZE, BUFFERS, COSTS OFF, TIMING OFF)
SELECT * FROM mv_daily_revenue ORDER BY day DESC LIMIT 7;

\echo ''
\echo '--- (c) Kết quả thật, 7 ngày gần nhất ---'
SELECT * FROM mv_daily_revenue ORDER BY day DESC LIMIT 7;

\echo ''
\echo '--- (d) Top 3 sản phẩm mỗi category (đã có sẵn rank, không tính lại) ---'
SELECT category, rank_in_category, product_name, units_sold, revenue
FROM mv_product_sales
WHERE rank_in_category <= 3
ORDER BY category, rank_in_category;

\echo ''
\echo '--- (e) KPI dashboard: 1 dòng, đọc là xong ---'
SELECT * FROM mv_dashboard_kpi;

\timing off
