#!/usr/bin/env bash
# KỊCH BẢN 5 — Lỗi ở nhánh async.
# Bài học: request thành công KHÔNG có nghĩa là công việc đã xong.
set -uo pipefail
cd "$(dirname "$0")" && . ./_lib.sh
require_gateway

header "Kịch bản 5: notification-svc hỏng (lỗi async)"

cat <<EOF

  Đây là kịch bản nguy hiểm nhất trong bốn kịch bản, vì nó VÔ HÌNH với client:

    - Client nhận HTTP 201. Đơn hàng đã CONFIRMED. Tồn kho đã trừ.
    - Nhưng vài trăm mili giây sau, notification-svc xử lý message thất bại
      và nack. Email xác nhận không bao giờ được gửi.
    - Không monitor nào ở tầng HTTP phát hiện được: tỉ lệ lỗi vẫn 0%.

  Distributed tracing bắt được vì span lỗi nằm CÙNG trace với request 201.

EOF

step "Đếm callback trước khi thử"
BEFORE="$(curl -sS "$GATEWAY_URL/stats" 2>/dev/null | (command -v jq >/dev/null && jq -r .callbacks_received || cat))"
note "callbacks_received = $BEFORE"

step "Gửi request có X-Fail-Mode: async_fail"
create_order "SKU-LAPTOP" 1 "async_fail"
report "Kết quả (chú ý: THÀNH CÔNG)"
TRACE_ASYNC="$TRACE_ID"

if [ "$HTTP_STATUS" = "201" ]; then
  good "HTTP 201 — nhìn từ phía client thì mọi thứ hoàn hảo"
fi

step "Chờ nhánh async chạy rồi kiểm tra lại"
sleep 4
AFTER="$(curl -sS "$GATEWAY_URL/stats" 2>/dev/null | (command -v jq >/dev/null && jq -r .callbacks_received || cat))"
note "callbacks_received = $AFTER"
if [ "$BEFORE" = "$AFTER" ]; then
  bad "Không có callback nào tới — công việc async đã hỏng, đúng như dự kiến"
else
  warn "Số callback có tăng — có thể là message tồn từ lần chạy trước"
fi

step "Log của notification-svc"
docker compose logs --tail 10 notification 2>/dev/null | sed 's/^/  /'

step "Đối chứng: một request bình thường thì callback phải tăng"
create_order "SKU-LAPTOP" 1
TRACE_OK="$TRACE_ID"
sleep 4
FINAL="$(curl -sS "$GATEWAY_URL/stats" 2>/dev/null | (command -v jq >/dev/null && jq -r .callbacks_received || cat))"
note "callbacks_received = $FINAL"
[ "$FINAL" != "$AFTER" ] && good "Callback đã tăng — nhánh async chạy đúng"

pause_for_export 3

lookfor \
  "QUAN TRỌNG: mở trace async, chờ 5 giây rồi TẢI LẠI trang. Span consumer đến muộn." \
  "Span gateway màu XANH (HTTP 201) nhưng span 'notification.render_email' màu ĐỎ." \
  "Đây là lý do 'trace có màu xanh' không đủ để kết luận request đã xong việc." \
  "Bung span 'order.created process' → tag lab.fail_mode=async_fail: baggage đã đi xuyên qua RabbitMQ." \
  "Trace đối chứng có thêm span HTTP POST /internal/callback nối ngược về gateway." \
  "Mở RabbitMQ UI (http://localhost:15673, lab/lab) → queue lab.order.created → xem số message bị nack."

cat <<EOF

  ${BOLD}Cách tìm loại lỗi này trong Jaeger${RESET}
    Search với Service = notification-svc và Tags = error=true
    → ra đúng những đơn hàng "thành công nhưng chưa gửi được email".
    Ở production, đây là truy vấn nên gắn vào alert.

  ${BOLD}Câu hỏi tự kiểm tra${RESET}
    1. Vì sao span của notification-svc lại xuất hiện MUỘN hơn trong Jaeger,
       dù cùng trace? (gợi ý: nó được ghi sau, và BatchSpanProcessor gửi theo lô)
    2. Message bị nack với requeue=false đi đâu? Cần thêm gì để không mất dữ liệu?
       (gợi ý: dead-letter exchange)
    3. Nếu amqpx.Publish inject traceparent TRƯỚC khi mở span producer thay vì sau,
       cây trace sẽ sai thế nào?

  Trace hỏng async : $JAEGER_URL/trace/$TRACE_ASYNC
  Trace đối chứng  : $JAEGER_URL/trace/$TRACE_OK

EOF
