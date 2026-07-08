#!/usr/bin/env bash
# Phase 6 — demo search feature API: highlight, facet+post_filter, synonym, autocomplete,
# zero-result fallback, multi-tenant filter, _source trim.
# Yêu cầu: docker stack + backend Phase 6 chạy ở :8090 (outbox mode).
#   cd backend && go run .
# Lần đầu áp mapping Phase 6: script tự chạy reindex + backfill (idempotent).
set -euo pipefail
B="${BACKEND:-http://localhost:8090}"

line() { printf '\n\033[1m== %s ==\033[0m\n' "$1"; }
pyget() { python3 -c "import sys,json;d=json.load(sys.stdin);print($1)"; }

line "0. áp mapping Phase 6 (reindex -> backfill), refresh _source có tenant_id + name.suggest"
curl -s -X POST "$B/admin/reindex"  >/dev/null
curl -s -X POST "$B/admin/backfill" | python3 -c "import sys,json;print('  backfilled=',json.load(sys.stdin)['backfilled'])"

line "1. seed 2 tenant (acme, globex) — tenant stamp từ header, KHÔNG từ body"
mk() { # $1=tenant $2=name $3=brand $4=category $5=price
  curl -s -X POST "$B/products" -H 'Content-Type: application/json' -H "X-Tenant-ID: $1" \
    -d "{\"name\":\"$2\",\"description\":\"$2 great device\",\"sku\":\"SKU-$RANDOM\",\"status\":\"active\",\"category\":\"$4\",\"brand\":\"$3\",\"price\":$5,\"in_stock\":true}" >/dev/null
}
mk acme   "Acme Laptop Pro"       Acme   laptop 1299
mk acme   "Budget Phone Y"        Budget phone  399
mk globex "Globex Notebook Air"   Globex laptop 1099
mk globex "Globex Phone Z"        Globex phone  799
sleep 2

line "2. Synonym (6.4): search 'notebook' ở tenant acme -> ra sản phẩm có chữ 'Laptop'"
curl -s "$B/search?q=notebook" -H 'X-Tenant-ID: acme' | pyget "'  total=',d['total'],' items=',[i['name'] for i in d['items']]"

line "3. Autocomplete (6.4): /suggest?q=glob (tenant globex)"
curl -s "$B/suggest?q=Globex" -H 'X-Tenant-ID: globex' | pyget "'  suggestions=',d['suggestions']"

line "4. Highlight (6.1): q=phone -> đoạn khớp bọc <em> (đã escape HTML)"
curl -s "$B/search?q=phone" -H 'X-Tenant-ID: acme' | pyget "'  highlight=',[i.get('highlight') for i in d['items']]"

line "5. Facet + post_filter (6.3): chọn brand=Acme -> hits chỉ Acme NHƯNG facet brand vẫn đủ trong tenant"
curl -s "$B/search?brand=Acme" -H 'X-Tenant-ID: acme' | pyget "'  hits_brands=',sorted({i['brand'] for i in d['items']}),' facet_brand=',[b['key'] for b in d['facets']['brand']]"

line "6. Multi-tenant (6.6): cùng q='phone', acme vs globex KHÔNG thấy data của nhau"
echo -n "  acme:   "; curl -s "$B/search?q=phone" -H 'X-Tenant-ID: acme'   | pyget "[i['name'] for i in d['items']]"
echo -n "  globex: "; curl -s "$B/search?q=phone" -H 'X-Tenant-ID: globex' | pyget "[i['name'] for i in d['items']]"

line "7. Zero-result fallback (6.5): q='labtopp xyz' typo -> fuzzy / did_you_mean"
curl -s "$B/search?q=labtopp" -H 'X-Tenant-ID: globex' | pyget "'  fallback=',d['fallback'],' did_you_mean=',d['did_you_mean'],' total=',d['total']"

line "8. _source trim (6.7): item không có 'description' (dành cho highlight), 'updated_at'"
curl -s "$B/search?q=phone" -H 'X-Tenant-ID: acme' | pyget "'  keys=',sorted((d['items'][0].keys()) if d['items'] else [])"

printf '\n\033[1mDONE.\033[0m Xem chi tiết query: queries/phase6-search-feature.http\n'
