#!/bin/bash
# Test suite cho cache-products, chạy theo đúng thứ tự README.
B=127.0.0.1:8020
PG="docker exec idempotency-postgres-1 psql -U postgres -d idempotency_lab -q -t"
RD="docker exec redis redis-cli"
PASS=0; FAIL=0

chk() { # chk "tên" "mong đợi" "thực tế"
  if [ "$2" == "$3" ]; then PASS=$((PASS+1)); printf '  PASS  %s\n' "$1"
  else FAIL=$((FAIL+1)); printf '  FAIL  %s\n        mong đợi: %s\n        thực tế : %s\n' "$1" "$2" "$3"; fi
}

echo "===== §1 Startup: seed + không warm-up ====="
chk "seed đúng số lượng" "2|7|24|42" \
  "$($PG -c "SELECT (SELECT count(*) FROM clients)||'|'||(SELECT count(*) FROM categories)||'|'||(SELECT count(*) FROM products)||'|'||(SELECT count(*) FROM product_sizes);" | tr -d ' \n')"
chk "Redis rỗng lúc start" "" "$($RD KEYS 'client:*:catalog' | tr -d '\r')"
chk "không có log warm-up/resync" "0" "$(grep -cE 'warm-up|resync' "$LOG")"
chk "health" '{"status":"ok"}' "$(curl -s $B/health)"

echo "===== §2 Cache-aside ====="
chk "lần 1 miss -> db + ttl" '"db" "1h0m0s"' "$(curl -s $B/clients/1001/catalog | jq -c '.source, .ttl' | tr '\n' ' ' | sed 's/ $//')"
chk "lần 2 hit -> cache" '"cache"' "$(curl -s $B/clients/1001/catalog | jq -c '.source')"
chk "TTL key = 3600" "3600" "$($RD TTL client:1001:catalog | tr -d '\r')"
chk "payload product đầu" '{"id":1,"name":"Trà Đào Cam Sả","sizes":[{"id":1,"name":"S","price":30000},{"id":2,"name":"M","price":40000},{"id":3,"name":"L","price":50000}]}' \
  "$(curl -s $B/clients/1001/catalog | jq -c '.catalog.categories[0].products[0]')"
chk "có cached_at" "true" "$(curl -s $B/clients/1001/catalog | jq -c '.catalog.cached_at != null')"

echo "===== §3 Data inactive không vào catalog ====="
chk "product inactive bị loại" "false" \
  "$(curl -s $B/clients/1001/catalog | jq -c '[.catalog.categories[].products[].name] | any(. == "Trà Sữa Ngưng Bán")')"
chk "category inactive bị loại" '["Pizza","Mì Ý","Nước"]' \
  "$(curl -s $B/clients/1002/catalog | jq -c '[.catalog.categories[].name]')"

echo "===== §4 Write invalidate cache ====="
chk "PUT size trả bản mới" '{"id":2,"product_id":1,"name":"M","price":45000,"sort_order":1}' \
  "$(curl -s -X PUT $B/clients/1001/sizes/2 -H 'Content-Type: application/json' -d '{"name":"M","price":45000}' | jq -c .)"
chk "key bị xoá sau write" "0" "$($RD EXISTS client:1001:catalog | tr -d '\r')"
chk "đọc lại -> db, giá mới" '"db" {"id":2,"name":"M","price":45000}' \
  "$(curl -s $B/clients/1001/catalog | jq -c '.source, .catalog.categories[0].products[0].sizes[1]' | tr '\n' ' ' | sed 's/ $//')"
chk "POST product (id 25, sizes 43/44)" '{"id":25,"client_id":"1001","category_id":1,"name":"Trà Ổi Hồng","active":true,"sizes":[{"id":43,"product_id":25,"name":"M","price":42000,"sort_order":0},{"id":44,"product_id":25,"name":"L","price":52000,"sort_order":1}]}' \
  "$(curl -s -X POST $B/clients/1001/products -H 'Content-Type: application/json' -d '{"category_id":1,"name":"Trà Ổi Hồng","sizes":[{"name":"M","price":42000},{"name":"L","price":52000}]}' | jq -c .)"
chk "create cũng xoá cache" "0" "$($RD EXISTS client:1001:catalog | tr -d '\r')"

echo "===== §5 Order đọc DB, không gửi expected_price ====="
chk "order 2 dòng, total 98000" \
  '{"lines":[{"size_id":2,"product_id":1,"name":"Trà Đào Cam Sả (M)","quantity":2,"unit_price":45000,"line_amount":90000},{"size_id":19,"product_id":10,"name":"Trân Châu Đen (Mặc định)","quantity":1,"unit_price":8000,"line_amount":8000}],"price_source":"db","total":98000}' \
  "$(curl -s -X POST $B/clients/1001/orders -H 'Content-Type: application/json' -d '{"items":[{"size_id":2,"quantity":2},{"size_id":19,"quantity":1}]}' | jq -c .)"

echo "===== §6 Giá đổi -> 409 ====="
curl -s $B/clients/1001/catalog > /dev/null   # warm cache, khớp DB
chk "409 khi lệch giá, cache đúng -> không xoá" \
  '{"catalog_refreshed":false,"changed_items":[{"size_id":2,"name":"Trà Đào Cam Sả (M)","expected_price":40000,"current_price":45000}],"error":"price_changed","message":"Giá đã thay đổi, vui lòng tải lại menu và xác nhận lại","new_total":45000}' \
  "$(curl -s -X POST $B/clients/1001/orders -H 'Content-Type: application/json' -d '{"items":[{"size_id":2,"quantity":1,"expected_price":40000}]}' | jq -c .)"
chk "cache KHÔNG bị xoá" "1" "$($RD EXISTS client:1001:catalog | tr -d '\r')"
chk "xác nhận lại giá mới -> 201" "45000" \
  "$(curl -s -X POST $B/clients/1001/orders -H 'Content-Type: application/json' -d '{"items":[{"size_id":2,"quantity":1,"expected_price":45000}]}' | jq -c '.total')"
chk "gom nhiều dòng lệch, bỏ dòng đúng giá" \
  '[{"size_id":1,"name":"Trà Đào Cam Sả (S)","expected_price":11,"current_price":30000},{"size_id":2,"name":"Trà Đào Cam Sả (M)","expected_price":22,"current_price":45000}] 128000' \
  "$(curl -s -X POST $B/clients/1001/orders -H 'Content-Type: application/json' -d '{"items":[{"size_id":1,"quantity":1,"expected_price":11},{"size_id":2,"quantity":2,"expected_price":22},{"size_id":19,"quantity":1,"expected_price":8000}]}' | jq -c '.changed_items, .new_total' | tr '\n' ' ' | sed 's/ $//')"

echo "----- cache lệch thật -> DEL -----"
$PG -c "UPDATE product_sizes SET price = 99000 WHERE id = 2;" > /dev/null
chk "409 + catalog_refreshed true" '"price_changed" 99000 true' \
  "$(curl -s -X POST $B/clients/1001/orders -H 'Content-Type: application/json' -d '{"items":[{"size_id":2,"quantity":1,"expected_price":45000}]}' | jq -c '.error, .changed_items[0].current_price, .catalog_refreshed' | tr '\n' ' ' | sed 's/ $//')"
chk "key đã bị xoá" "0" "$($RD EXISTS client:1001:catalog | tr -d '\r')"
chk "log ghi rõ lý do lệch" "1" "$(grep -c 'giá cache 45000 != DB 99000' "$LOG")"
chk "GET lại -> db" '"db"' "$(curl -s $B/clients/1001/catalog | jq -c '.source')"

echo "----- món bị tắt, cache vẫn còn -----"
$PG -c "UPDATE products SET active = false WHERE id = 1;" > /dev/null
chk "400 + catalog_refreshed true" '{"catalog_refreshed":true,"error":"size 2 is not available"}' \
  "$(curl -s -X POST $B/clients/1001/orders -H 'Content-Type: application/json' -d '{"items":[{"size_id":2,"quantity":1}]}' | jq -c .)"
$PG -c "UPDATE products SET active = true WHERE id = 1;" -c "UPDATE product_sizes SET price = 45000 WHERE id = 2;" > /dev/null

echo "===== §7 Cache hỏng -> degrade ====="
curl -s $B/clients/1001/catalog > /dev/null
$RD SET client:1001:catalog 'không phải JSON' > /dev/null
chk "payload hỏng -> vẫn 200, source db" '"db"' "$(curl -s $B/clients/1001/catalog | jq -c '.source')"
chk "log báo payload hỏng" "1" "$(grep -c 'payload corrupted' "$LOG")"

echo "===== §8 Delete ====="
chk "delete product 25 -> 204" "204" "$(curl -s -o /dev/null -w '%{http_code}' -X DELETE $B/clients/1001/products/25)"
chk "sizes của product 25 bị xoá theo" "0" "$($PG -c 'SELECT count(*) FROM product_sizes WHERE product_id = 25;' | tr -d ' \n')"

echo "===== §9 Case lỗi ====="
chk "404 client lạ" '{"error":"client_not_found"}' "$(curl -s $B/clients/9999/catalog)"
chk "400 product không size" '{"error":"at least one size is required"}' \
  "$(curl -s -X POST $B/clients/1001/products -H 'Content-Type: application/json' -d '{"category_id":1,"name":"X","sizes":[]}')"
chk "400 category cross-tenant" '{"error":"category not found"}' \
  "$(curl -s -X POST $B/clients/1001/products -H 'Content-Type: application/json' -d '{"category_id":4,"name":"X","sizes":[{"name":"M","price":1000}]}')"
chk "404 size cross-tenant" '{"error":"size not found"}' \
  "$(curl -s -X PUT $B/clients/1001/sizes/28 -H 'Content-Type: application/json' -d '{"name":"S","price":1000}')"
chk "400 order món inactive" '{"catalog_refreshed":false,"error":"size 18 is not available"}' \
  "$(curl -s -X POST $B/clients/1001/orders -H 'Content-Type: application/json' -d '{"items":[{"size_id":18,"quantity":1}]}')"
chk "404 delete cross-tenant" "404" "$(curl -s -o /dev/null -w '%{http_code}' -X DELETE $B/clients/1001/products/17)"
chk "400 order rỗng" '{"error":"items are required"}' \
  "$(curl -s -X POST $B/clients/1001/orders -H 'Content-Type: application/json' -d '{"items":[]}')"
chk "400 quantity âm" '{"error":"size_id and quantity must be positive"}' \
  "$(curl -s -X POST $B/clients/1001/orders -H 'Content-Type: application/json' -d '{"items":[{"size_id":2,"quantity":-1}]}')"
chk "400 json hỏng" '{"error":"invalid_json"}' \
  "$(curl -s -X POST $B/clients/1001/orders -H 'Content-Type: application/json' -d 'not json')"
chk "400 id không hợp lệ" '{"error":"invalid sizeID"}' \
  "$(curl -s -X PUT $B/clients/1001/sizes/abc -H 'Content-Type: application/json' -d '{"name":"S","price":1000}')"

echo
echo "================ KẾT QUẢ: $PASS pass, $FAIL fail ================"
exit $FAIL
