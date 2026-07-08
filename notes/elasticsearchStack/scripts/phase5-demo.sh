#!/usr/bin/env bash
# Phase 5 — demo end-to-end luồng sync DB -> ES (outbox + recovery).
# Yêu cầu: docker stack đang chạy + backend chạy ở :8090 (outbox mode).
#   cd backend && go run .        # terminal khác
set -euo pipefail
B="${BACKEND:-http://localhost:8090}"

jqp() { python3 -c "import sys,json;print(json.dumps(json.load(sys.stdin)))"; }
line() { printf '\n\033[1m== %s ==\033[0m\n' "$1"; }

line "0. health"
curl -s "$B/healthz"; echo

line "1. create product (outbox) -> worker sync -> search thấy"
curl -s -X POST "$B/products" -H 'Content-Type: application/json' \
  -d '{"name":"Sony Alpha A7 IV","description":"full frame mirrorless camera","sku":"SONY-A7IV","status":"active","category":"camera","brand":"Sony","price":2499,"in_stock":true}' | jqp
sleep 2
echo -n "  search a7: "; curl -s "$B/search?q=alpha" | python3 -c "import sys,json;print('total=',json.load(sys.stdin)['total'])"

line "2. DRIFT: tắt ES rồi create -> parked trong outbox"
docker stop es-learn >/dev/null
curl -s -X POST "$B/products" -H 'Content-Type: application/json' \
  -d '{"name":"Canon EOS R6","description":"mirrorless camera","sku":"CANON-R6","status":"active","category":"camera","brand":"Canon","price":1999,"in_stock":true}' >/dev/null
sleep 2
echo -n "  reconcile (ES down): "; curl -s "$B/admin/reconcile"; echo

line "3. RECOVERY: bật ES -> worker tự drain outbox"
docker start es-learn >/dev/null
for i in $(seq 1 20); do curl -sf localhost:9200/_cluster/health >/dev/null 2>&1 && break; sleep 2; done
sleep 4
echo -n "  reconcile: "; curl -s "$B/admin/reconcile"; echo

line "4. backfill idempotent + reindex zero-downtime"
curl -s -X POST "$B/admin/backfill"; echo
curl -s -X POST "$B/admin/reindex"; echo
echo -n "  reconcile cuối: "; curl -s "$B/admin/reconcile"; echo

printf '\n\033[1mDONE.\033[0m Đổi WRITE_MODE=dual RUN_WORKER=false để xem dual-write KHÔNG hồi phục.\n'
