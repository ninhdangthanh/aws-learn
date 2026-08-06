#!/usr/bin/env bash
# KỊCH BẢN 1 — Đường đi thành công. Đây là baseline để so sánh với 4 kịch bản lỗi.
set -uo pipefail
cd "$(dirname "$0")" && . ./_lib.sh
require_gateway

header "Kịch bản 1: Luồng thành công (baseline)"

cat <<EOF

  Một request đi qua cả bốn service và cả bốn loại I/O:

    curl ──HTTP──> gateway ──gRPC──> order-svc ──SQL──> Postgres
                                        │
                                        ├──gRPC──> inventory-svc ──SQL──> Postgres
                                        │
                                        └──AMQP──> RabbitMQ
                                                      │
                                                      └──> notification-svc ──HTTP──> gateway

EOF

step "Đặt 1 chiếc SKU-LAPTOP"
create_order "SKU-LAPTOP" 1
report "Đơn đã tạo"

pause_for_export 3

lookfor \
  "Chọn Service = gateway, tìm trace vừa tạo (hoặc mở thẳng link ở trên)." \
  "Đếm số service tham gia: phải đủ 4 — gateway, order-svc, inventory-svc, notification-svc." \
  "Bung span 'order.created publish' → tab Tags → xem lab.injected_traceparent." \
  "So sánh với span 'order.created process' → lab.received_traceparent. Hai giá trị PHẢI trùng phần trace-id." \
  "Nhìn timeline: span của notification-svc bắt đầu SAU khi span gateway đã kết thúc." \
  "Bung span Postgres → tab Tags → db.statement chứa nguyên văn câu SQL."

cat <<EOF

  ${BOLD}Câu hỏi tự kiểm tra${RESET}
    1. Vì sao span của notification-svc lại nằm cùng trace, dù nó chạy sau khi
       client đã nhận response? (gợi ý: đọc internal/amqpx/carrier.go)
    2. Trace này có bao nhiêu span? Span nào là span gốc?
    3. Thử xoá header traceparent ở amqpx.Publish rồi chạy lại — chuyện gì xảy ra?

EOF
