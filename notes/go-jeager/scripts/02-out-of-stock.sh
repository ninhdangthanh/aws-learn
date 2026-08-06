#!/usr/bin/env bash
# KỊCH BẢN 2 — Lỗi nghiệp vụ ở service sâu nhất.
# Bài học: span đỏ đầu tiên bạn nhìn thấy KHÔNG phải root cause.
set -uo pipefail
cd "$(dirname "$0")" && . ./_lib.sh
require_gateway

header "Kịch bản 2: Hết hàng (lỗi nghiệp vụ)"

cat <<EOF

  inventory-svc trả về gRPC code FAILED_PRECONDITION. Lỗi này lan ngược lên
  qua 3 tầng: inventory → order → gateway → client (HTTP 409).

  Trong Jaeger, CẢ BA span đều đỏ. Người mới thường nhìn span đỏ trên cùng
  (gateway) rồi kết luận gateway hỏng. Sai. Phải lần xuống span đỏ SÂU NHẤT.

EOF

step "Cách A — hết hàng thật: đặt 99 chiếc SKU-RARE (kho chỉ có 2)"
create_order "SKU-RARE" 99
report "Kết quả"
TRACE_A="$TRACE_ID"

step "Cách B — ép lỗi bằng fail mode, dù kho vẫn còn hàng"
note "Header X-Fail-Mode: out_of_stock → baggage → chảy xuống tận inventory-svc"
create_order "SKU-LAPTOP" 1 "out_of_stock"
report "Kết quả"
TRACE_B="$TRACE_ID"

pause_for_export 3

lookfor \
  "Mở trace: có ĐÚNG 4 span bị đánh dấu đỏ, xếp lồng nhau qua 3 service." \
  "Span đỏ SÂU NHẤT là inventory.v1.InventoryService/Reserve (của inventory-svc) → root cause." \
  "Bung span đó → tab Tags → lab.qty_available và lab.qty_requested cho biết chính xác thiếu bao nhiêu." \
  "Tab Logs của span → sự kiện 'exception' do span.RecordError sinh ra, kèm nguyên văn lỗi." \
  "Ở span gateway → tag lab.http_status=409 và lab.grpc_code=FailedPrecondition." \
  "So trace B với trace A: B có thêm tag lab.out_of_stock_forced=true và lab.fail_mode=out_of_stock."

cat <<EOF

  ${BOLD}${YELLOW}Chi tiết dễ khiến bạn bỏ sót${RESET}
    Span GỐC 'POST /orders' của gateway KHÔNG màu đỏ, dù request trả về 409.

    Lý do: otelhttp chỉ đánh dấu span lỗi khi status >= 500. Mã 4xx được coi là
    "client sai", không phải "server hỏng" — đúng theo đặc tả OpenTelemetry.

    Hệ quả rất thực tế: nếu bạn lọc Jaeger bằng Service=gateway + error=true thì
    KHÔNG tìm ra trace này. Phải lọc theo service ở tầng dưới, hoặc thêm tag
    nghiệp vụ của riêng mình (lab.http_status=409) rồi lọc theo tag đó.
EOF

cat <<EOF

  ${BOLD}Mẹo tìm kiếm trong Jaeger${RESET}
    Ô 'Tags' ở màn hình Search, gõ:  lab.sku=SKU-RARE
    hoặc:                            error=true
    → lọc ra đúng những trace hỏng, không phải cuộn tay.

  ${BOLD}Câu hỏi tự kiểm tra${RESET}
    1. Nếu inventory-svc chỉ gọi span.RecordError mà quên span.SetStatus(codes.Error),
       span có còn hiện màu đỏ không? (đọc hàm fail() trong cmd/inventory/main.go)
    2. Vì sao gateway trả 409 chứ không phải 500? (đọc writeGRPCError trong cmd/gateway/main.go)
    3. Đơn hàng lúc này ở trạng thái nào trong DB? Kiểm tra bằng: make psql

  Trace A: $JAEGER_URL/trace/$TRACE_A
  Trace B: $JAEGER_URL/trace/$TRACE_B

EOF
