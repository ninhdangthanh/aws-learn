#!/usr/bin/env bash
# Phase 5.4b — ordering / stale write: bản cũ KHÔNG được đè bản mới.
# Mô phỏng index đến trễ, đảo thứ tự: sau khi ES đã có v2, ta nhét thẳng vào outbox
# một "ý định index" bản v1 với version nhỏ. Worker gửi lên ES -> ES trả 409
# (external version) -> giữ nguyên v2. Yêu cầu: backend chạy :8090 (outbox), pg-learn up.
set -euo pipefail
B="${BACKEND:-http://localhost:8090}"
PSQL=(docker exec -i pg-learn psql -U app -d shop -qtAX)

echo "== 1. create v1 =="
ID=$(curl -s -X POST "$B/products" -H 'Content-Type: application/json' \
  -d '{"name":"StaleGuard v1","description":"first","sku":"SG-1","status":"active","category":"test","brand":"Acme","price":100,"in_stock":true}' \
  | python3 -c "import sys,json;print(json.load(sys.stdin)['id'])")
echo "  id=$ID"; sleep 2

echo "== 2. update -> v2 (bản mới nhất, hợp lệ) =="
curl -s -o /dev/null -X PUT "$B/products/$ID" -H 'Content-Type: application/json' \
  -d '{"name":"StaleGuard v2","description":"newer","sku":"SG-1","status":"active","category":"test","brand":"Acme","price":200,"in_stock":true}'
sleep 2
echo -n "  ES hiện tại: "; curl -s "localhost:9200/products/_doc/$ID?_source=name,price" | python3 -c "import sys,json;s=json.load(sys.stdin)['_source'];print(s['name'],s['price'])"

echo "== 3. NHÉT stale index v1 vào outbox với version=1 (đảo thứ tự, đến trễ) =="
"${PSQL[@]}" <<SQL
INSERT INTO outbox (aggregate, aggregate_id, op, version, payload)
VALUES ('product', '$ID', 'index', 1,
  '{"id":$ID,"name":"StaleGuard v1 (STALE)","description":"first","sku":"SG-1","status":"active","category":"test","brand":"Acme","price":100,"in_stock":true,"created_at":"2020-01-01T00:00:00Z","updated_at":"2020-01-01T00:00:00Z"}'::jsonb);
SQL
echo "  đã chèn outbox stale (version=1)"; sleep 2

echo "== 4. KIỂM: ES phải VẪN là v2 (bản cũ bị 409 từ chối) =="
NAME=$(curl -s "localhost:9200/products/_doc/$ID?_source=name,price" | python3 -c "import sys,json;s=json.load(sys.stdin)['_source'];print(s['name'],'|',s['price'])")
echo "  ES = $NAME"
case "$NAME" in
  "StaleGuard v2 | 200"*) echo "  PASS: bản cũ KHÔNG đè bản mới (external version hoạt động)";;
  *) echo "  FAIL: ES bị ghi đè bằng bản cũ!"; exit 1;;
esac
echo -n "  outbox stale đã processed (worker không kẹt): "
"${PSQL[@]}" -c "SELECT count(*) FROM outbox WHERE aggregate_id='$ID' AND processed_at IS NULL;"

echo "== 5. cleanup =="
curl -s -o /dev/null -X DELETE "$B/products/$ID"
echo "  DONE"
