#!/usr/bin/env bash
# KỊCH BẢN 4 — Service chết giữa chừng.
# Bài học: SPAN THIẾU cũng là một tín hiệu chẩn đoán, không kém gì span đỏ.
set -uo pipefail
cd "$(dirname "$0")" && . ./_lib.sh
require_gateway

header "Kịch bản 4: inventory-svc panic và chết"

cat <<EOF

  inventory-svc gọi panic() giữa lúc xử lý RPC. grpc-go không recover panic,
  nên cả process chết. Hệ quả dây chuyền:

    - BatchSpanProcessor chưa kịp flush ⇒ span của inventory-svc MẤT TRẮNG.
    - order-svc thấy kết nối đứt ⇒ nhận gRPC code Unavailable.
    - gateway dịch thành HTTP 503.

  Trong Jaeger bạn sẽ thấy một trace CỤT: có span client gọi sang inventory-svc,
  nhưng KHÔNG có span server tương ứng. Đó chính là chữ ký của một service chết.

EOF

step "Trạng thái inventory-svc trước khi thử"
docker compose ps inventory --format 'table {{.Service}}\t{{.Status}}' 2>/dev/null | sed 's/^/  /'

step "Gửi request có X-Fail-Mode: panic"
create_order "SKU-PHONE" 1 "panic"
report "Kết quả"
TRACE_PANIC="$TRACE_ID"

step "Log của inventory-svc lúc chết"
docker compose logs --tail 12 inventory 2>/dev/null | sed 's/^/  /'

step "Chờ Docker dựng lại container (restart: unless-stopped)"
for i in $(seq 1 20); do
  sleep 2
  if curl -sS -o /dev/null --max-time 3 "$GATEWAY_URL/healthz" 2>/dev/null; then
    create_order "SKU-PHONE" 1
    if [ "$HTTP_STATUS" = "201" ]; then
      good "inventory-svc đã hồi phục sau ~$((i * 2))s"
      report "Request kiểm chứng sau khi hồi phục"
      TRACE_RECOVER="$TRACE_ID"
      break
    fi
  fi
  printf '  %s...đang chờ (%ds)%s\r' "$DIM" "$((i * 2))" "$RESET"
done
echo

pause_for_export 3

lookfor \
  "Mở trace panic. Đếm số service: chỉ có 2 (gateway, order-svc) — inventory-svc BIẾN MẤT." \
  "Span 'inventory.v1.InventoryService/Reserve' màu đỏ là span CLIENT của order-svc, không phải span server." \
  "Bung nó ra: không có span con nào. Đây là dấu hiệu 'service phía dưới không kịp báo cáo'." \
  "Tag lab.grpc_code=Unavailable ở gateway phân biệt trường hợp này với 409 của kịch bản 2." \
  "So với trace hồi phục: trace đó có đủ cả span server của inventory-svc."

cat <<EOF

  ${BOLD}Vì sao span bị mất — và cách khắc phục trong thực tế${RESET}
    BatchSpanProcessor gom span vào buffer rồi mới gửi theo lô. Process chết
    đột ngột thì buffer bay theo. Ba cách giảm thiểu:

      1. recover() trong interceptor gRPC → span vẫn được ghi và flush.
      2. SimpleSpanProcessor (gửi ngay từng span) — an toàn hơn nhưng rất tốn.
      3. Đặt OTel Collector cạnh service (sidecar/agent) để rút ngắn quãng mất mát.

    Thử tự làm cách 1: thêm grpc.UnaryInterceptor có recover vào cmd/inventory/main.go,
    chạy lại script này, và xem span có xuất hiện không.

  ${BOLD}Câu hỏi tự kiểm tra${RESET}
    1. Làm sao phân biệt "service chết" với "service chậm đến mức timeout"
       chỉ bằng cách nhìn trace?
    2. Đơn hàng vừa rồi ở trạng thái nào trong DB? Vì sao? (đọc markFailed trong cmd/order/main.go)

  Trace panic    : $JAEGER_URL/trace/$TRACE_PANIC
  Trace hồi phục : $JAEGER_URL/trace/${TRACE_RECOVER:-<chưa hồi phục>}

EOF
