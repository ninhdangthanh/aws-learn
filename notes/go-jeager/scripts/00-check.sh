#!/usr/bin/env bash
# Kiểm tra mọi thành phần đã sẵn sàng trước khi chạy các kịch bản.
set -uo pipefail
cd "$(dirname "$0")" && . ./_lib.sh

header "Kiểm tra hệ thống"

check() {
  local name="$1" url="$2"
  if curl -sS -o /dev/null --max-time 5 "$url" 2>/dev/null; then
    good "$name"
  else
    bad "$name  ($url không phản hồi)"
    FAILED=1
  fi
}

FAILED=0

step "Hạ tầng"
check "Jaeger UI        " "$JAEGER_URL/"
check "RabbitMQ UI      " "http://localhost:15673/"

step "Service ứng dụng"
check "gateway /healthz " "$GATEWAY_URL/healthz"

step "Postgres"
if docker exec lab-postgres pg_isready -U lab -d lab >/dev/null 2>&1; then
  good "postgres sẵn sàng"
  printf '\n  Tồn kho hiện tại:\n'
  docker exec lab-postgres psql -U lab -d lab -c \
    'SELECT sku, name, qty FROM inventory ORDER BY sku;' 2>/dev/null | sed 's/^/    /'
else
  bad "postgres chưa sẵn sàng"
  FAILED=1
fi

step "Container đang chạy"
docker compose ps --format 'table {{.Service}}\t{{.Status}}' 2>/dev/null | sed 's/^/  /'

if [ "$FAILED" -eq 0 ]; then
  printf '\n%s✓ Tất cả sẵn sàng. Chạy tiếp: ./scripts/01-happy.sh%s\n\n' "$GREEN" "$RESET"
else
  printf '\n%s✗ Có thành phần chưa lên. Xem log: make logs%s\n\n' "$RED" "$RESET"
  exit 1
fi
