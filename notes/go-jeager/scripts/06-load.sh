#!/usr/bin/env bash
# KỊCH BẢN 6 — Sinh lưu lượng hỗn hợp để tập dùng màn hình Search của Jaeger.
# Bài học: tìm outlier giữa đám đông, thay vì soi từng trace một.
set -uo pipefail
cd "$(dirname "$0")" && . ./_lib.sh
require_gateway

TOTAL="${1:-40}"

header "Kịch bản 6: Lưu lượng hỗn hợp ($TOTAL request)"

cat <<EOF

  Trộn request tốt và xấu theo tỉ lệ gần với thực tế:

    ~70%  thành công
    ~15%  hết hàng      (409)
    ~10%  DB chậm       (201 nhưng ~3s)
    ~5%   hỏng async    (201 nhưng email không gửi được)

  Không có panic trong bộ này — panic làm chết service, không hợp để đo tải.

EOF

SKUS="SKU-LAPTOP SKU-PHONE SKU-MOUSE"
ok=0; conflict=0; slow=0; async=0; other=0

step "Đang chạy..."
for i in $(seq 1 "$TOTAL"); do
  # shellcheck disable=SC2086
  sku=$(echo $SKUS | tr ' ' '\n' | sed -n "$(( (RANDOM % 3) + 1 ))p")
  roll=$(( RANDOM % 100 ))

  if   [ "$roll" -lt 70 ]; then mode="";             label="ok"
  elif [ "$roll" -lt 85 ]; then mode="out_of_stock"; label="conflict"
  elif [ "$roll" -lt 95 ]; then mode="slow_db";      label="slow"
  else                          mode="async_fail";   label="async"
  fi

  create_order "$sku" 1 "$mode" "load-$i"

  case "$label" in
    ok)       ok=$((ok + 1)) ;;
    conflict) conflict=$((conflict + 1)) ;;
    slow)     slow=$((slow + 1)) ;;
    async)    async=$((async + 1)) ;;
    *)        other=$((other + 1)) ;;
  esac

  printf '  %s[%d/%d]%s %-12s %-14s → HTTP %s\n' \
    "$DIM" "$i" "$TOTAL" "$RESET" "$sku" "${mode:-<bình thường>}" "$HTTP_STATUS"
done

pause_for_export 5

cat <<EOF

  ${BOLD}Tổng kết${RESET}
    thành công : $ok
    hết hàng   : $conflict
    DB chậm    : $slow
    hỏng async : $async
    khác       : $other

EOF

lookfor \
  "Vào $JAEGER_URL, Service = gateway, Limit Results = 100, bấm Find Traces." \
  "Biểu đồ scatter phía trên: trục dọc là latency. Chấm nằm cao chính là các request slow_db." \
  "Bấm thẳng vào một chấm cao để mở trace đó — nhanh hơn nhiều so với cuộn danh sách." \
  "Lọc lỗi: ô Tags gõ  error=true  → chỉ còn trace hỏng." \
  "Lọc theo latency: ô Min Duration gõ  2s  → chỉ còn request chậm." \
  "Lọc theo nghiệp vụ: Tags gõ  lab.sku=SKU-PHONE  → đây là lợi ích của việc gắn attribute nghiệp vụ." \
  "Đổi Service sang notification-svc + Tags error=true → tìm ra lỗi async mà tầng HTTP không thấy."

cat <<EOF

  ${BOLD}Thử thêm${RESET}
    - Tab 'System Architecture' → 'DAG': Jaeger tự vẽ sơ đồ phụ thuộc giữa 4 service
      từ chính dữ liệu trace. Không ai khai báo tay sơ đồ này cả.
    - Chọn 2 trace bất kỳ trong danh sách → nút 'Compare' để xem chênh lệch cấu trúc.

EOF
