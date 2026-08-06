#!/usr/bin/env bash
# Chạy tuần tự cả 5 kịch bản, dừng giữa mỗi kịch bản để bạn kịp xem Jaeger.
set -uo pipefail
cd "$(dirname "$0")" && . ./_lib.sh

header "Chạy toàn bộ lab"

note "Sau mỗi kịch bản, script dừng lại. Mở link trace được in ra, xem xong thì Enter."
echo

./00-check.sh || exit 1

for s in 01-happy 02-out-of-stock 03-slow-db 04-panic 05-async-fail; do
  printf '\n%s%s Nhấn Enter để chạy %s.sh (hoặc Ctrl-C để dừng)%s ' "$BOLD" "$CYAN" "$s" "$RESET"
  read -r _
  "./$s.sh"
done

printf '\n%s%s Nhấn Enter để sinh lưu lượng hỗn hợp (kịch bản 6)%s ' "$BOLD" "$CYAN" "$RESET"
read -r _
./06-load.sh 40

header "Xong. Giờ mở $JAEGER_URL và tự khám phá."
