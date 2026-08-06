#!/usr/bin/env bash
# Hàm dùng chung cho mọi script kịch bản. Không chạy trực tiếp file này.

GATEWAY_URL="${GATEWAY_URL:-http://localhost:8080}"
JAEGER_URL="${JAEGER_URL:-http://localhost:16686}"

if [ -t 1 ]; then
  BOLD=$'\033[1m'; DIM=$'\033[2m'; RED=$'\033[31m'; GREEN=$'\033[32m'
  YELLOW=$'\033[33m'; BLUE=$'\033[34m'; CYAN=$'\033[36m'; RESET=$'\033[0m'
else
  BOLD=''; DIM=''; RED=''; GREEN=''; YELLOW=''; BLUE=''; CYAN=''; RESET=''
fi

# Ba biến này được create_order gán, script gọi xong đọc lại.
HTTP_STATUS=''
TRACE_ID=''
BODY=''

# Không dùng khung có viền phải: printf '%-60s' đếm theo BYTE chứ không theo bề
# rộng hiển thị, nên dấu tiếng Việt (ký tự UTF-8 nhiều byte) làm lệch viền.
header() {
  printf '\n%s══════════════════════════════════════════════════════════════%s\n' "$BLUE" "$RESET"
  printf '  %s%s%s\n' "$BOLD" "$1" "$RESET"
  printf '%s══════════════════════════════════════════════════════════════%s\n' "$BLUE" "$RESET"
}

step()   { printf '\n%s▸ %s%s\n' "$CYAN" "$1" "$RESET"; }
note()   { printf '  %s%s%s\n' "$DIM" "$1" "$RESET"; }
good()   { printf '  %s✓ %s%s\n' "$GREEN" "$1" "$RESET"; }
bad()    { printf '  %s✗ %s%s\n' "$RED" "$1" "$RESET"; }
warn()   { printf '  %s! %s%s\n' "$YELLOW" "$1" "$RESET"; }

# Định dạng JSON nếu có jq, không thì in thô.
pretty() {
  if command -v jq >/dev/null 2>&1; then jq . 2>/dev/null || cat; else cat; fi
}

# create_order <sku> <qty> [fail_mode] [customer]
#
# Gọi POST /orders rồi gán HTTP_STATUS, TRACE_ID, BODY.
create_order() {
  local sku="$1" qty="$2" mode="${3:-}" customer="${4:-lab-user}"
  local hdrfile bodyfile
  hdrfile="$(mktemp)"; bodyfile="$(mktemp)"

  local args=(-sS -X POST "$GATEWAY_URL/orders"
              -H "Content-Type: application/json"
              -d "{\"sku\":\"$sku\",\"qty\":$qty,\"customer\":\"$customer\"}")
  if [ -n "$mode" ]; then
    args+=(-H "X-Fail-Mode: $mode")
  fi

  HTTP_STATUS="$(curl "${args[@]}" -D "$hdrfile" -o "$bodyfile" -w '%{http_code}' || echo 000)"
  # tr -d '\r' vì header HTTP kết thúc bằng CRLF, không xoá thì trace id dính \r
  # và link Jaeger sẽ hỏng.
  TRACE_ID="$(grep -i '^x-trace-id:' "$hdrfile" | tr -d '\r' | awk '{print $2}')"
  BODY="$(cat "$bodyfile")"
  rm -f "$hdrfile" "$bodyfile"
}

# report [nhãn] — in kết quả lần gọi gần nhất kèm link Jaeger.
report() {
  local label="${1:-Kết quả}"
  printf '\n  %s%s%s\n' "$BOLD" "$label" "$RESET"
  printf '  HTTP status : '
  if [ "$HTTP_STATUS" -ge 200 ] 2>/dev/null && [ "$HTTP_STATUS" -lt 300 ]; then
    printf '%s%s%s\n' "$GREEN" "$HTTP_STATUS" "$RESET"
  else
    printf '%s%s%s\n' "$RED" "$HTTP_STATUS" "$RESET"
  fi
  printf '  Response    : %s\n' "$(printf '%s' "$BODY" | pretty | tr '\n' ' ' | sed 's/  */ /g')"
  trace_link
}

trace_link() {
  if [ -n "$TRACE_ID" ]; then
    printf '  %sTrace       : %s/trace/%s%s\n' "$BOLD" "$JAEGER_URL" "$TRACE_ID" "$RESET"
  else
    warn "Không nhận được X-Trace-Id (gateway có đang chạy không?)"
  fi
}

# lookfor <dòng> ... — in phần "trong Jaeger bạn sẽ thấy gì".
lookfor() {
  printf '\n  %s%sCần quan sát trong Jaeger:%s\n' "$BOLD" "$YELLOW" "$RESET"
  local line
  for line in "$@"; do
    printf '    %s•%s %s\n' "$YELLOW" "$RESET" "$line"
  done
}

require_gateway() {
  if ! curl -sS -o /dev/null --max-time 3 "$GATEWAY_URL/healthz" 2>/dev/null; then
    bad "Không gọi được gateway tại $GATEWAY_URL"
    note "Chạy 'make up' trước, rồi 'make ps' để chắc chắn mọi container đã lên."
    exit 1
  fi
}

pause_for_export() {
  local seconds="${1:-3}"
  note "Chờ ${seconds}s cho BatchSpanProcessor đẩy span sang Jaeger..."
  sleep "$seconds"
}
