#!/usr/bin/env bash
# =====================================================================
# demo.sh — chạy các kịch bản demo materialized view
#   ./demo.sh speed    so tốc độ view thường vs matview
#   ./demo.sh stale    dữ liệu cũ + refresh tay
#   ./demo.sh lock     REFRESH khoá reader vs CONCURRENTLY thì không
#   ./demo.sh inspect  soi kích thước / lịch sử refresh
#   ./demo.sh watch    xem refresher 30s tự cập nhật dữ liệu
#   ./demo.sh frozen   chứng minh Postgres KHÔNG tự refresh
#   ./demo.sh all      chạy speed + stale + lock + inspect
# =====================================================================
set -euo pipefail

CT=matview_pg
PSQL="docker exec -i $CT psql -U app -d matview_lab -v ON_ERROR_STOP=1"
DIR="$(cd "$(dirname "$0")" && pwd)"

hr()  { printf '\n\033[1;34m%s\033[0m\n' "==================== $* ===================="; }

need_db() {
    docker exec "$CT" pg_isready -U app -d matview_lab >/dev/null 2>&1 || {
        echo "Postgres chưa chạy. Chạy: docker compose up -d" >&2
        exit 1
    }
}

demo_speed()   { hr "DEMO 1: pre-aggregate";      $PSQL -f - < "$DIR/sql/004-demo-speed.sql"; }
demo_stale()   { hr "DEMO 2: dữ liệu cũ";         $PSQL -f - < "$DIR/sql/005-demo-staleness.sql"; }
demo_inspect() { hr "DEMO 4: soi matview";        $PSQL -f - < "$DIR/sql/006-inspect.sql"; }

demo_lock() {
    hr "DEMO 3: REFRESH khoá reader, CONCURRENTLY thì không"

    echo "--- (a) REFRESH thường: giữ ACCESS EXCLUSIVE LOCK 6 giây ---"
    docker exec -d "$CT" psql -U app -d matview_lab \
        -c "BEGIN; REFRESH MATERIALIZED VIEW mv_daily_revenue; SELECT pg_sleep(6); COMMIT;"
    sleep 1
    echo "    session 2 chạy SELECT... (sẽ bị treo tới khi refresh xong)"
    $PSQL -c "\timing on" -c "SELECT count(*) FROM mv_daily_revenue;"

    sleep 1
    echo ""
    echo "--- (b) REFRESH CONCURRENTLY: reader chạy bình thường ---"
    docker exec -d "$CT" psql -U app -d matview_lab \
        -c "BEGIN; REFRESH MATERIALIZED VIEW CONCURRENTLY mv_daily_revenue; SELECT pg_sleep(6); COMMIT;"
    sleep 1
    echo "    session 2 chạy SELECT... (trả về ngay, đọc bản cũ)"
    $PSQL -c "\timing on" -c "SELECT count(*) FROM mv_daily_revenue;"
    sleep 6
}

demo_frozen() {
    hr "DEMO 6: Postgres KHÔNG tự refresh"
    echo "--- Tắt refresher ---"
    docker compose -f "$DIR/docker-compose.yml" stop refresher >/dev/null 2>&1

    echo "--- Thêm 1 đơn 777.000.000đ ---"
    $PSQL -q -c "
        WITH o AS (
            INSERT INTO orders (customer_id, status, created_at)
            VALUES (1, 'paid', now()) RETURNING id
        )
        INSERT INTO order_items (order_id, product_id, qty, unit_price)
        SELECT o.id, 1, 1, 777000000 FROM o;"

    echo "--- Chờ 60s: matview đứng im, chờ bao lâu cũng vậy ---"
    for i in $(seq 1 6); do poll_line; sleep 10; done

    echo ""
    echo "--- Bật lại refresher ---"
    docker compose -f "$DIR/docker-compose.yml" start refresher >/dev/null 2>&1
    for i in $(seq 1 5); do poll_line; sleep 8; done

    echo ""
    echo "=> Chu kỳ 30s đến từ refresher.sh, KHÔNG phải từ Postgres."
    echo "   Postgres không có auto-refresh. Matview chỉ đổi khi có ai đó"
    echo "   chạy REFRESH MATERIALIZED VIEW."
}

poll_line() {
    $PSQL -qtAX -c "
        SELECT to_char(now(), 'HH24:MI:SS')
            || ' | matview='  || coalesce((SELECT revenue::text FROM mv_daily_revenue
                                           WHERE day = (now() AT TIME ZONE 'UTC')::date), 'null')
            || ' | realtime=' || coalesce((SELECT revenue::text FROM v_daily_revenue
                                           WHERE day = (now() AT TIME ZONE 'UTC')::date), 'null');"
}

demo_watch() {
    hr "DEMO 5: refresher 30s"
    docker ps --format '{{.Names}}' | grep -q matview_refresher || {
        echo "Container refresher chưa chạy. Chạy: docker compose up -d" >&2
        exit 1
    }

    echo "--- Thêm 1 đơn 500.000.000đ, KHÔNG refresh tay ---"
    $PSQL -q -c "
        WITH o AS (
            INSERT INTO orders (customer_id, status, created_at)
            VALUES (1, 'paid', now()) RETURNING id
        )
        INSERT INTO order_items (order_id, product_id, qty, unit_price)
        SELECT o.id, 1, 1, 500000000 FROM o;"

    echo "--- Poll 5s/lần trong 40s, chờ refresher nuốt dữ liệu mới ---"
    for i in $(seq 1 8); do poll_line; sleep 5; done
    echo ""
    echo "=> Hai con số lệch nhau tối đa ~30s rồi tự khớp lại."
}

need_db
case "${1:-all}" in
    speed)   demo_speed ;;
    stale)   demo_stale ;;
    lock)    demo_lock ;;
    inspect) demo_inspect ;;
    watch)   demo_watch ;;
    frozen)  demo_frozen ;;
    all)     demo_speed; demo_stale; demo_lock; demo_inspect ;;
    *)       sed -n '2,11p' "$0" ; exit 1 ;;
esac
