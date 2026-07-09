#!/usr/bin/env bash
#
# PreToolUse | matcher: Write|Edit
# Chặn Claude ghi secret cứng (hardcoded) vào source của MFA-example.
#
# Đây là project demo MFA: TOTP secret, session token, WebAuthn key đều là
# dữ liệu nhạy cảm. Hook này deny NGAY TRƯỚC khi file bị ghi, nên secret
# không bao giờ chạm vào đĩa (khác PostToolUse — lúc đó đã ghi rồi).
#
set -uo pipefail

input=$(cat)

file=$(printf '%s' "$input" | jq -r '.tool_input.file_path // empty')
# Write dùng .content, Edit dùng .new_string
content=$(printf '%s' "$input" | jq -r '.tool_input.content // .tool_input.new_string // empty')

[[ -z "$file" || -z "$content" ]] && exit 0

# Chỉ can thiệp vào file nằm trong MFA-example.
# Suy ra từ vị trí script (.claude/hooks/x.sh -> lên 2 cấp), nên đổi tên
# thư mục project vẫn chạy đúng.
PROJECT_ROOT=$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)
[[ "$file" == "$PROJECT_ROOT"/* ]] || exit 0

case "$file" in
  *.js | *.mjs | *.cjs | *.json | *.html) ;;
  *) exit 0 ;;
esac

# Giá trị mẫu thì bỏ qua — tránh deny nhầm khi Claude viết ví dụ/placeholder.
PLACEHOLDER="your|example|changeme|placeholder|dummy|sample|xxxx|process\.env|\\\$\{|<"

findings=""
scan() { # $1 = mô tả, $2 = regex
  local hits
  # -e bắt buộc: có pattern bắt đầu bằng '-' (private key header), không có -e
  # thì grep hiểu nhầm là option.
  hits=$(printf '%s' "$content" | grep -nEi -e "$2" | grep -vEi -e "$PLACEHOLDER" | head -3) || true
  [[ -n "$hits" ]] && findings+="  • $1"$'\n'"$(printf '%s' "$hits" | sed 's/^/      /')"$'\n'
  return 0
}

scan "TOTP/MFA secret dạng base32 viết cứng" \
  "(mfa_?secret|totp_?secret|otp_?secret)[[:space:]]*[:=][[:space:]]*[\"'][A-Z2-7]{16,}[\"']"

scan "Mật khẩu viết cứng" \
  "password[[:space:]]*[:=][[:space:]]*[\"'][^\"']{8,}[\"']"

scan "API key / client secret / auth token viết cứng" \
  "(api[_-]?key|apikey|client[_-]?secret|auth[_-]?token|bearer)[[:space:]]*[:=][[:space:]]*[\"'][^\"' ]{8,}[\"']"

scan "Token/secret dạng hex dài viết cứng" \
  "(token|secret)[[:space:]]*[:=][[:space:]]*[\"'][0-9a-f]{32,}[\"']"

scan "AWS access key id" \
  "AKIA[0-9A-Z]{16}"

scan "Private key nhúng thẳng vào file" \
  "-----BEGIN [A-Z ]*PRIVATE KEY-----"

if [[ -n "$findings" ]]; then
  reason="Chặn ghi secret cứng vào $(basename "$file"):"$'\n'"$findings"$'\n'
  reason+="Dùng biến môi trường (process.env.X) hoặc sinh runtime bằng crypto/speakeasy thay vì literal."
  jq -n --arg r "$reason" '{
    hookSpecificOutput: {
      hookEventName: "PreToolUse",
      permissionDecision: "deny",
      permissionDecisionReason: $r
    }
  }'
  exit 0
fi

exit 0
