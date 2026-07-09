#!/usr/bin/env bash
#
# PreToolUse | matcher: Bash
# Chặn vài lệnh phá hoại / không hồi phục được.
#
# Lưu ý: hook này KHÔNG phải rào chắn bảo mật chống lại kẻ tấn công —
# nó chỉ chống tai nạn. Bất cứ lệnh nào cũng có thể viết lách qua regex.
#
set -uo pipefail

input=$(cat)
cmd=$(printf '%s' "$input" | jq -r '.tool_input.command // empty')
[[ -z "$cmd" ]] && exit 0

deny() {
  jq -n --arg r "$1" '{
    hookSpecificOutput: {
      hookEventName: "PreToolUse",
      permissionDecision: "deny",
      permissionDecisionReason: $r
    }
  }'
  exit 0
}

# `-e` bắt buộc: pattern bắt đầu bằng '-' (vd --force-with-lease) sẽ bị grep
# hiểu là option và báo lỗi. Lỗi đó in ra stdout -> phá JSON trả về.
# Ngoài ra `grep` ở đây có thể là ugrep/ripgrep alias, nên bám sát POSIX.
m() { printf '%s' "$cmd" | grep -qEi -e "$1"; }

# rm -rf kèm đường dẫn nguy hiểm (/, .., ~, glob)
if m 'rm[[:space:]]+(-[a-z]*r[a-z]*f|-[a-z]*f[a-z]*r)' \
  && m '([[:space:]]/([[:space:]]|$)|\.\.|~|\*)'; then
  deny "Lệnh 'rm -rf' với đường dẫn nguy hiểm (/, .., ~, *) bị chặn. Xóa từng đường dẫn tương đối, cụ thể."
fi

# force push ghi đè lịch sử remote (--force-with-lease thì cho qua)
if m 'git[[:space:]]+push' \
  && m '(--force([[:space:]]|$)|[[:space:]]-f([[:space:]]|$))' \
  && ! m '--force-with-lease'; then
  deny "Force push bị chặn. Dùng --force-with-lease nếu thật sự cần ghi đè."
fi

# publish nhầm project demo lên npm registry
if m 'npm[[:space:]]+publish'; then
  deny "'npm publish' bị chặn: MFA-example là project học, không phải package để phát hành."
fi

# curl | sh — thực thi script tải từ mạng
if m '(curl|wget)[^|]*\|[[:space:]]*(sudo[[:space:]]+)?(ba|z|k)?sh'; then
  deny "Pipe từ curl/wget thẳng vào shell bị chặn. Tải file về, đọc nội dung, rồi mới chạy."
fi

# chmod 777
if m 'chmod[[:space:]]+(-[a-z]+[[:space:]]+)*777'; then
  deny "'chmod 777' bị chặn. Cấp quyền tối thiểu cần thiết (vd 755 cho script, 644 cho file thường)."
fi

exit 0
