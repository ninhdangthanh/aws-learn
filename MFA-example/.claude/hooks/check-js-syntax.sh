#!/usr/bin/env bash
#
# PostToolUse | matcher: Write|Edit
# Chạy `node --check` trên file .js vừa sửa. Project này không có linter/test,
# nên đây là lưới an toàn rẻ nhất: bắt lỗi cú pháp ngay lúc sửa.
#
# PostToolUse không hủy được thao tác (file đã ghi), nhưng exit 2 đẩy stderr
# ngược lại cho Claude đọc như một lỗi -> Claude tự sửa ngay trong lượt đó.
#
set -uo pipefail

input=$(cat)
file=$(printf '%s' "$input" | jq -r '.tool_input.file_path // empty')

[[ -z "$file" ]] && exit 0

case "$file" in
  *.js | *.cjs | *.mjs) ;;
  *) exit 0 ;;
esac

PROJECT_ROOT=$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)
[[ "$file" == "$PROJECT_ROOT"/* ]] || exit 0
[[ -f "$file" ]] || exit 0

if ! err=$(node --check "$file" 2>&1); then
  {
    echo "Lỗi cú pháp JavaScript trong ${file#"$PROJECT_ROOT"/} sau khi sửa:"
    echo
    echo "$err"
    echo
    echo "Hãy sửa lại file này trước khi làm việc khác."
  } >&2
  exit 2
fi

exit 0
