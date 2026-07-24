#!/usr/bin/env bash
# Phase 2 — (re)tạo index `products` với mapping tường minh + bulk seed 30 document.
# Idempotent: xóa index cũ rồi tạo lại, _id = product id nên chạy lại an toàn.
set -euo pipefail

ES="${ES_URL:-http://localhost:9200}"
DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
INDEX="${INDEX:-products}"

echo "==> DELETE /$INDEX (nếu tồn tại)"
curl -s -o /dev/null -w "  http %{http_code}\n" -X DELETE "$ES/$INDEX"

echo "==> CREATE /$INDEX với mapping"
curl -s -X PUT "$ES/$INDEX" \
  -H 'Content-Type: application/json' \
  --data-binary "@$DIR/products.mapping.json" | sed 's/^/  /'
echo

echo "==> BULK index products.bulk.ndjson"
resp=$(curl -s -X POST "$ES/$INDEX/_bulk" \
  -H 'Content-Type: application/x-ndjson' \
  --data-binary "@$DIR/products.bulk.ndjson")
# _bulk luôn trả 200; phải soi field "errors"
if echo "$resp" | grep -q '"errors":true'; then
  echo "  BULK có lỗi:"; echo "$resp" | head -c 800; echo; exit 1
fi
echo "  errors: false"

echo "==> Refresh + count"
curl -s -o /dev/null -X POST "$ES/$INDEX/_refresh"
count=$(curl -s "$ES/$INDEX/_count" | sed -n 's/.*"count":\([0-9]*\).*/\1/p')
echo "  count = $count"
[ "$count" -ge 1 ] || { echo "FAIL: index rỗng"; exit 1; }

echo "==> Phase 2 seed DONE (index=$INDEX, docs=$count)"
