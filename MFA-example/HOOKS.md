# Hooks của MFA-example

Bốn hook đặt trong `.claude/`, viết bằng bash + `jq`, không thêm dependency nào.

```
.claude/
├── settings.json            # khai báo hook
└── hooks/
    ├── guard-secrets.sh     # PreToolUse  | Write|Edit  → deny secret viết cứng
    ├── guard-bash.sh        # PreToolUse  | Bash        → deny lệnh phá hoại
    ├── check-js-syntax.sh   # PostToolUse | Write|Edit  → node --check
    └── session-context.sh   # SessionStart              → bơm trạng thái project
```

---

## ⚠️ Đọc trước: hook chỉ chạy khi project root là `MFA-example`

Claude Code **chỉ đọc `.claude/settings.json` ở project root**, không đọc từ thư mục con.
`MFA-example` là thư mục con của repo `aws-learn`, nên:

| Bạn mở Claude Code ở đâu | Hook có chạy không? |
|---|:---|
| `cd MFA-example && claude` | ✅ Có |
| `cd aws-learn && claude` | ❌ Không — `aws-learn` mới là root |

Muốn dùng hook khi mở ở `aws-learn`, copy block `hooks` từ `MFA-example/.claude/settings.json` sang
`aws-learn/.claude/settings.json` và sửa đường dẫn thành:

```json
"command": "\"$CLAUDE_PROJECT_DIR/MFA-example/.claude/hooks/guard-secrets.sh\""
```

Các script đều tự suy ra thư mục `MFA-example` từ vị trí của chính nó và **bỏ qua mọi file nằm
ngoài đó**, nên copy sang root vẫn an toàn: hook sẽ không đụng tới phần còn lại của repo.

---

## Bốn hook làm gì

### 1. `guard-secrets.sh` — chặn secret viết cứng

`PreToolUse` trên `Write|Edit`. Đây là project MFA, nên TOTP secret / session token / private key
lọt vào source là rủi ro thật. Chạy ở `PreToolUse` nên nó **deny trước khi file được ghi** — secret
không bao giờ chạm đĩa. (`PostToolUse` thì đã ghi rồi, chỉ báo lại được.)

Bắt: TOTP secret base32, password literal, api key / client secret / auth token, token hex ≥32 ký tự,
AWS access key id, header private key.

Bỏ qua nếu dòng đó chứa từ mẫu (`your`, `example`, `changeme`, `process.env`, `${`, …) — để Claude vẫn
viết được placeholder và ví dụ.

Chỉ quét `.js`, `.mjs`, `.cjs`, `.json`, `.html`.

### 2. `guard-bash.sh` — chặn lệnh phá hoại

`PreToolUse` trên `Bash`. Deny: `rm -rf` kèm `/`, `..`, `~` hoặc glob; `git push --force`
(nhưng cho qua `--force-with-lease`); `npm publish`; `curl … | sh`; `chmod 777`.

Đây là rào chắn **chống tai nạn**, không phải rào chắn bảo mật. Regex nào cũng lách được.

### 3. `check-js-syntax.sh` — `node --check` sau khi sửa

`PostToolUse` trên `Write|Edit`. Project không có test/linter nên đây là lưới an toàn rẻ nhất.

`PostToolUse` **không hủy được** thao tác (file đã ghi rồi), nhưng `exit 2` đẩy stderr ngược lại cho
Claude đọc như một lỗi → Claude sửa ngay trong lượt đó. Đó là toàn bộ giá trị của hook này.

### 4. `session-context.sh` — bơm trạng thái vào đầu session

`SessionStart`. Trả `additionalContext`: `node_modules` đã cài chưa, cổng 3000 có ai chiếm không, và
nhắc lại các ràng buộc bất biến (store in-memory, `rpID`/`origin` phải khớp, password plaintext là
chủ ý). Nhờ vậy Claude không phải dò lại bằng `ls`/`cat`, và không "sửa" nhầm những thứ cố ý.

---

## Cơ chế: hook nói chuyện với Claude Code thế nào

**Vào**: JSON qua **stdin** — `tool_name`, `tool_input`, `cwd`, `session_id`, `hook_event_name`…
`Write` dùng `tool_input.content`, `Edit` dùng `tool_input.new_string`. Lấy field bằng `jq -r`.

**Ra**, có hai cách:

*Exit code* — `0` là qua; `2` là **chặn**, stderr được đưa cho Claude đọc; khác `0`/`2` chỉ là cảnh
báo cho bạn, không chặn. `check-js-syntax.sh` dùng cách này.

*JSON trên stdout* — kiểm soát kỹ hơn. `guard-secrets.sh` và `guard-bash.sh` dùng cách này:

```json
{
  "hookSpecificOutput": {
    "hookEventName": "PreToolUse",
    "permissionDecision": "deny",
    "permissionDecisionReason": "lý do — Claude sẽ đọc và đổi hướng"
  }
}
```

`permissionDecision` nhận `allow` | `deny` | `ask`, chỉ dành cho `PreToolUse`.
`SessionStart` thì trả `additionalContext` thay vì quyết định.

Chỉ `PreToolUse`, `UserPromptSubmit`, `Stop`, `SubagentStop`, `PreCompact` là chặn được.
`PostToolUse` thì **không** — nó chỉ phản hồi ngược.

---

## Test hook bằng tay

Hook đọc stdin, nên gọi thẳng được, không cần chạy Claude Code:

```bash
cd MFA-example

# Kỳ vọng: deny
jq -n '{tool_name:"Bash",tool_input:{command:"git push --force origin main"}}' \
  | .claude/hooks/guard-bash.sh

# Kỳ vọng: deny + lý do
jq -n --arg f "$PWD/backend/server.js" --arg c 'const password = "hunter2abc";' \
  '{tool_name:"Write",tool_input:{file_path:$f,content:$c}}' \
  | .claude/hooks/guard-secrets.sh

# Kỳ vọng: im lặng, exit 0 (server.js thật không chứa secret cứng)
jq -n --arg f "$PWD/backend/server.js" --arg c "$(cat backend/server.js)" \
  '{tool_name:"Write",tool_input:{file_path:$f,content:$c}}' \
  | .claude/hooks/guard-secrets.sh; echo "exit=$?"

# Kỳ vọng: JSON additionalContext
echo '{"hook_event_name":"SessionStart"}' | .claude/hooks/session-context.sh | jq .
```

Không có output + `exit=0` nghĩa là **cho qua**. Đó là trạng thái đúng cho code hiện tại.

Sau khi sửa `settings.json`, gõ `/hooks` trong Claude Code để xem hook nào đang được nạp.
Hook được đọc lúc khởi động session — sửa xong nên khởi động lại session.

---

## Hai cái bẫy đã gặp khi viết mấy hook này

**`grep` và pattern bắt đầu bằng `-`.** `grep -qE '--force-with-lease'` làm grep hiểu `--force-with-lease`
là *option* rồi báo lỗi và in usage ra stdout — đúng cái stdout mà Claude Code đang chờ đọc JSON.
Kết quả: hook "im lặng qua" một cách vô hình. Luôn dùng `grep -e "$pattern"`.

Bẫy này càng dễ dính vì trên máy này `grep` thực ra là **ugrep** (alias), thông báo lỗi khác hẳn GNU grep.

**Bắt buộc test với input thật.** Bản đầu của `guard-secrets.sh` có luật bắt private key nhưng nó
**chưa bao giờ chạy** (cùng lỗi `-` ở trên) — chỉ lộ ra khi test bằng một private key thật.
Và một luật quét secret quá tay sẽ deny nhầm chính `server.js` (file này có chữ `password` khắp nơi).
Hãy luôn chạy hook lên source thật và xác nhận nó **im lặng**.

---

## Bảo mật

Hook chạy với **đầy đủ quyền user của bạn, không sandbox, không hỏi xác nhận**. Coi việc thêm một
hook nghiêm túc như thêm một dòng vào `.zshrc`.

`.claude/settings.json` nằm trong repo và **được commit** → ai clone repo này rồi mở bằng Claude Code
sẽ chạy các script trên. Ngược lại: hãy **đọc `.claude/hooks/` của repo lạ trước khi mở nó**.

---

## Xem thêm

- Giải thích hook tổng quát → [`../notes/claude-docs/hooks-guide.md`](../notes/claude-docs/hooks-guide.md)
- Chọn giữa hook / skill / CLAUDE.md → [`../notes/claude-docs/writing-skills-and-planning.md`](../notes/claude-docs/writing-skills-and-planning.md)
