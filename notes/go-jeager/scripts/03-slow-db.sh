#!/usr/bin/env bash
# KỊCH BẢN 3 — Lỗi hạ tầng: truy vấn chậm.
# Bài học: đọc waterfall để tìm span nuốt thời gian, thay vì đoán mò.
set -uo pipefail
cd "$(dirname "$0")" && . ./_lib.sh
require_gateway

header "Kịch bản 3: Truy vấn Postgres chậm (3 giây)"

cat <<EOF

  Không có lỗi nào cả — request trả về 201 thành công. Chỉ là chậm.

  Đây là loại sự cố khó chịu nhất khi không có tracing: user báo "hệ thống lag",
  log không có dòng ERROR nào, và bạn không biết chậm ở đâu trong 4 service.

EOF

step "Bước 1: đo baseline (không bơm lỗi)"
T0=$(date +%s%N)
create_order "SKU-MOUSE" 1
T1=$(date +%s%N)
FAST_MS=$(( (T1 - T0) / 1000000 ))
report "Request bình thường"
good "Thời gian: ${FAST_MS}ms"
TRACE_FAST="$TRACE_ID"

step "Bước 2: bơm pg_sleep(3) vào inventory-svc"
note "X-Fail-Mode: slow_db"
T0=$(date +%s%N)
create_order "SKU-MOUSE" 1 "slow_db"
T1=$(date +%s%N)
SLOW_MS=$(( (T1 - T0) / 1000000 ))
report "Request chậm"
warn "Thời gian: ${SLOW_MS}ms  (chậm hơn baseline $(( SLOW_MS - FAST_MS ))ms)"
TRACE_SLOW="$TRACE_ID"

pause_for_export 3

lookfor \
  "Mở trace chậm. Thanh span 'inventory.slow_lookup' chiếm gần trọn chiều ngang (~3013ms)." \
  "Bung nó ra → span con 'query SELECT pg_sleep(\$1)' do otelpgx sinh tự động, cũng ~3013ms." \
  "Tag lab.injected_delay_seconds=3 xác nhận đây là độ trễ cố ý." \
  "Chú ý: span gateway và order-svc cũng dài ~3037ms, nhưng chúng chỉ đang CHỜ." \
  "Thời gian thực sự tiêu tốn nằm ở span LÁ — đó là nguyên tắc đọc critical path." \
  "So 3013ms (pg_sleep) với 3037ms (gateway): 24ms chênh lệch là chi phí thật của 3 chặng mạng."

cat <<EOF

  ${BOLD}So sánh hai trace cạnh nhau${RESET}
    Jaeger có tính năng Compare rất hợp lúc này:
      1. Mở  $JAEGER_URL/trace/$TRACE_SLOW
      2. Menu góc phải trên → 'Compare'
      3. Dán trace id nhanh vào ô thứ hai: $TRACE_FAST
    → Jaeger tô màu chênh lệch, chỉ thẳng span nào phình ra.

  ${BOLD}Câu hỏi tự kiểm tra${RESET}
    1. Nhìn waterfall, span nào là "thủ phạm" và span nào chỉ là "nạn nhân đang chờ"?
    2. Nếu tăng seconds trong slowQuery() lên 10, request có timeout không?
       Timeout đặt ở đâu? (gợi ý: context.WithTimeout trong cmd/inventory/main.go)
    3. Mở tab 'Trace Statistics' của Jaeger — service nào tốn nhiều self-time nhất?
       Self-time khác Duration ở chỗ nào?

  Trace nhanh: $JAEGER_URL/trace/$TRACE_FAST
  Trace chậm: $JAEGER_URL/trace/$TRACE_SLOW

EOF
