-- =====================================================================
-- DEMO 4 — Soi matview: đã populate chưa, nặng bao nhiêu, refresh lúc nào
-- =====================================================================

\echo '--- Danh sách matview + trạng thái populate ---'
SELECT
    matviewname,
    ispopulated,
    hasindexes,
    pg_size_pretty(pg_total_relation_size(('public.' || matviewname)::regclass)) AS size
FROM pg_matviews
WHERE schemaname = 'public'
ORDER BY matviewname;

\echo ''
\echo '--- So sánh dung lượng bảng gốc vs matview ---'
SELECT
    relname,
    CASE relkind WHEN 'r' THEN 'table' WHEN 'm' THEN 'matview' END AS kind,
    pg_size_pretty(pg_total_relation_size(oid)) AS size
FROM pg_class
WHERE relnamespace = 'public'::regnamespace
  AND relkind IN ('r', 'm')
  AND relname IN ('orders', 'order_items', 'mv_daily_revenue',
                  'mv_product_sales', 'mv_dashboard_kpi')
ORDER BY pg_total_relation_size(oid) DESC;

\echo ''
\echo '--- 10 lần refresh gần nhất (Postgres không tự lưu, do ta tự log) ---'
SELECT
    view_name,
    to_char(refreshed_at, 'HH24:MI:SS') AS at,
    duration_ms,
    is_concurrent,
    round(extract(epoch FROM now() - refreshed_at))::int AS seconds_ago
FROM matview_refresh_log
ORDER BY refreshed_at DESC
LIMIT 10;

\echo ''
\echo '--- Trung bình thời gian refresh theo view ---'
SELECT
    view_name,
    count(*)                          AS runs,
    round(avg(duration_ms), 1)        AS avg_ms,
    round(max(duration_ms), 1)        AS max_ms
FROM matview_refresh_log
GROUP BY view_name
ORDER BY avg_ms DESC;
