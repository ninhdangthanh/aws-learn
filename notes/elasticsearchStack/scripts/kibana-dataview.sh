#!/usr/bin/env bash
# Phase 4 — tạo Kibana Data View `products` (timestamp = created_at) qua API.
# Idempotent: nếu đã tồn tại, Kibana trả lỗi "Duplicate" -> coi như OK.
set -euo pipefail
KB="${KB_URL:-http://localhost:5601}"

echo "==> Create data view products"
resp=$(curl -s -X POST "$KB/api/data_views/data_view" \
  -H 'kbn-xsrf: true' -H 'Content-Type: application/json' \
  -d '{"data_view":{"name":"products","title":"products","timeFieldName":"created_at"}}')

if echo "$resp" | grep -q '"data_view"'; then
  echo "$resp" | python3 -c "import sys,json;dv=json.load(sys.stdin)['data_view'];print('  OK id=%s'%dv['id'])"
elif echo "$resp" | grep -qi 'duplicate'; then
  echo "  Data view đã tồn tại (OK)"
else
  echo "  resp: $resp"; exit 1
fi

echo "==> Data view sẵn sàng. Mở Kibana -> Dashboard -> Create để dựng bar + line chart."
