#!/usr/bin/env bash
#
# SessionStart
# Bơm trạng thái thật của project vào context ngay đầu session, để Claude
# không phải dò lại bằng ls/cat (và không đoán sai).
#
# SessionStart không chặn được gì; nó chỉ cung cấp context.
#
set -uo pipefail

cat >/dev/null # nuốt stdin, hook này không cần đọc

PROJECT_ROOT=$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)

ctx="MFA-example — Express + TOTP (speakeasy) + WebAuthn passkeys, frontend vanilla JS."

if [[ -d "$PROJECT_ROOT/backend/node_modules" ]]; then
  ctx+=$'\n'"- backend/node_modules: đã cài."
else
  ctx+=$'\n'"- backend/node_modules: CHƯA cài. Cần 'npm install' trong backend/ trước khi chạy server."
fi

if lsof -ti:3000 >/dev/null 2>&1; then
  ctx+=$'\n'"- Cổng 3000: ĐANG có tiến trình lắng nghe (server có thể đã chạy sẵn — đừng khởi động trùng)."
else
  ctx+=$'\n'"- Cổng 3000: đang trống (server chưa chạy)."
fi

ctx+=$'\n'"- Ràng buộc đã biết của server.js: store là in-memory (users/sessions/tempSessions mất khi restart);"
ctx+=$'\n'"  rpID='localhost' và origin='http://localhost:3000' phải khớp nhau, nếu lệch thì WebAuthn fail;"
ctx+=$'\n'"  password đang lưu plaintext — chủ ý của demo, đừng 'sửa' trừ khi được yêu cầu."

jq -n --arg c "$ctx" '{
  hookSpecificOutput: {
    hookEventName: "SessionStart",
    additionalContext: $c
  }
}'
