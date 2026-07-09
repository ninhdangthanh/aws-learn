# Hooks — ép hành vi tự động "mỗi lần X thì làm Y"

> Tách từ `claude-operation.md`. Giải thích hook là gì, các sự kiện, cách hook giao tiếp với Claude Code,
> và ví dụ thật đang chạy trên máy này.
> Chọn giữa hook / skill / CLAUDE.md → xem `writing-skills-and-planning.md` phần C.

---

## 1. Hook là gì?

**Hook = một lệnh shell được Claude Code tự chạy tại một thời điểm định trước** trong vòng đời session (trước khi gọi tool, sau khi gọi tool, khi bạn gửi prompt, khi Claude trả lời xong...).

Điểm mấu chốt: **harness chạy hook, không phải Claude**. Nên hook là cách *duy nhất* để đảm bảo một hành vi **luôn luôn** xảy ra — không phụ thuộc vào việc Claude có "nhớ" làm hay không.

> Hook là tính năng của **Claude Code (CLI)**. Bản web claude.ai không có hook.

Dùng hook khi bạn muốn:

| Nhu cầu | Hook nào |
|---------|----------|
| Chạy formatter/linter sau khi Claude sửa file | `PostToolUse` |
| Chặn Claude chạy lệnh nguy hiểm (`rm -rf`, commit vào `main`) | `PreToolUse` |
| Viết lại lệnh trước khi chạy (vd tiết kiệm token) | `PreToolUse` |
| Bơm thêm context mỗi khi user gửi prompt | `UserPromptSubmit` |
| Ghi log / gửi notification mỗi lần Claude xong việc | `Stop` |

---

## 2. Khai báo ở đâu?

Trong `settings.json` — cùng thứ tự ưu tiên như các setting khác:

| File | Phạm vi |
|------|---------|
| `~/.claude/settings.json` | Global, mọi project |
| `<repo>/.claude/settings.json` | Chỉ repo đó, **commit được** để share team |
| `<repo>/.claude/settings.local.json` | Chỉ máy cá nhân, không commit |

Cấu trúc chung:

```json
{
  "hooks": {
    "<Tên sự kiện>": [
      {
        "matcher": "Bash",
        "hooks": [
          { "type": "command", "command": "lệnh-shell-của-bạn" }
        ]
      }
    ]
  }
}
```

- `matcher`: lọc theo **tên tool**, hỗ trợ regex (`Edit|Write`). Bỏ trống hoặc `"*"` = mọi tool.
- Mỗi entry `hooks` có thể thêm `timeout` (giây) và `statusMessage` (dòng chữ hiện lúc hook chạy).

> Không muốn sửa tay JSON → gõ `/hooks` trong Claude Code để xem/sửa bằng giao diện.

---

## 3. Các sự kiện có thể hook vào

| Sự kiện | Kích hoạt khi | Chặn được? |
|---------|---------------|:----------:|
| `PreToolUse` | Trước khi một tool chạy | ✅ allow / deny / ask |
| `PostToolUse` | Sau khi tool chạy xong | ❌ (nhưng phản hồi ngược lại cho Claude đọc) |
| `UserPromptSubmit` | Bạn vừa gửi prompt, trước khi Claude xử lý | ✅ |
| `SessionStart` | Mở session mới hoặc resume | ❌ |
| `SessionEnd` | Đóng session | ❌ |
| `Stop` | Claude (agent chính) trả lời xong | ✅ bắt làm tiếp |
| `SubagentStop` | Một subagent trả lời xong | ✅ |
| `PreCompact` | Trước khi context bị nén | ❌ |
| `Notification` | Claude Code xin quyền hoặc đang idle | ❌ |

`matcher` chủ yếu dùng cho `PreToolUse` / `PostToolUse` (lọc theo tên tool). Các sự kiện khác không có tool để lọc nên thường bỏ `matcher`.

---

## 4. Hook nhận & trả dữ liệu thế nào?

### Đầu vào — JSON qua **stdin**

```json
{
  "session_id": "abc123",
  "transcript_path": "/Users/.../transcript.jsonl",
  "cwd": "/Users/you/project",
  "hook_event_name": "PreToolUse",
  "tool_name": "Bash",
  "tool_input": { "command": "git status" }
}
```

- `PostToolUse` có thêm `tool_response` (kết quả tool trả về).
- `UserPromptSubmit` có `prompt` thay cho `tool_*`.
- Biến môi trường `$CLAUDE_PROJECT_DIR` trỏ tới root repo — dùng nó để gọi script trong repo.

Lấy field ra bằng `jq`:

```bash
file=$(jq -r '.tool_input.file_path')
```

### Đầu ra — cách 1: exit code

| Exit code | Ý nghĩa |
|:---------:|---------|
| `0` | Thành công. Với `UserPromptSubmit` và `SessionStart`, **stdout được thêm thẳng vào context** của Claude. Sự kiện khác thì stdout chỉ hiện trong transcript. |
| `2` | **Chặn**. stderr được đưa lại cho Claude đọc như một lỗi → viết lý do vào stderr để Claude tự sửa. |
| khác | Lỗi không chặn, chỉ cảnh báo cho user. |

### Đầu ra — cách 2: JSON trên stdout (kiểm soát kỹ hơn)

```json
{
  "hookSpecificOutput": {
    "hookEventName": "PreToolUse",
    "permissionDecision": "deny",
    "permissionDecisionReason": "Không được commit thẳng vào main"
  }
}
```

Các field hay dùng:

- `permissionDecision`: `allow` | `deny` | `ask` (chỉ `PreToolUse`).
- `additionalContext`: chuỗi bơm thêm vào context (`UserPromptSubmit`, `SessionStart`).
- `continue: false` + `stopReason`: dừng hẳn lượt của Claude.
- `suppressOutput: true`: không hiện stdout trong transcript.

---

## 5. Ví dụ

### Tự động format sau khi Claude sửa file Go

```json
{
  "hooks": {
    "PostToolUse": [
      {
        "matcher": "Edit|Write",
        "hooks": [
          {
            "type": "command",
            "command": "jq -r '.tool_input.file_path' | grep '\\.go$' | xargs -r gofmt -w"
          }
        ]
      }
    ]
  }
}
```

### Chặn commit thẳng vào `main`

Script `~/.claude/hooks/block-main-commit.sh`:

```bash
#!/usr/bin/env bash
cmd=$(jq -r '.tool_input.command')
branch=$(git rev-parse --abbrev-ref HEAD 2>/dev/null)

if [[ "$cmd" == git\ commit* && "$branch" == "main" ]]; then
  echo "Đang ở nhánh main. Tạo feature branch trước khi commit." >&2
  exit 2   # exit 2 = chặn, stderr được Claude đọc
fi
exit 0
```

```json
{
  "hooks": {
    "PreToolUse": [
      {
        "matcher": "Bash",
        "hooks": [
          { "type": "command", "command": "~/.claude/hooks/block-main-commit.sh" }
        ]
      }
    ]
  }
}
```

---

## 6. Hook đang chạy thật trên máy này

Trích `~/.claude/settings.json`:

```json
{
  "hooks": {
    "PreToolUse": [
      {
        "matcher": "Bash",
        "hooks": [{ "type": "command", "command": "rtk hook claude" }]
      },
      {
        "matcher": "Grep|Glob|Bash",
        "hooks": [{
          "type": "command",
          "command": "node '/Users/dangthanhninh/.claude/hooks/gitnexus/gitnexus-hook.cjs'",
          "timeout": 10,
          "statusMessage": "Enriching with GitNexus graph context..."
        }]
      }
    ],
    "PostToolUse": [
      {
        "matcher": "Bash",
        "hooks": [{
          "type": "command",
          "command": "node '/Users/dangthanhninh/.claude/hooks/gitnexus/gitnexus-hook.cjs'",
          "timeout": 10,
          "statusMessage": "Checking GitNexus index freshness..."
        }]
      }
    ]
  }
}
```

| Hook | Làm gì |
|------|--------|
| `rtk hook claude` (PreToolUse/Bash) | Viết lại `git status` → `rtk git status` để tiết kiệm token. Claude **không biết** lệnh đã bị sửa — đúng nghĩa "transparent, 0 tokens overhead" trong `RTK.md`. |
| GitNexus (PreToolUse) | Bơm thêm context từ code graph trước khi search |
| GitNexus (PostToolUse) | Kiểm tra index của code graph còn tươi không |

---

## 7. Bảo mật — đọc trước khi thêm hook

- Hook chạy với **đầy đủ quyền của user bạn**, **không sandbox**, **không hỏi xác nhận**.
- Một hook viết ẩu (sai `xargs`, sai glob) có thể xóa file thật.
- Coi việc thêm hook nghiêm túc như thêm dòng vào `.zshrc`.
- Hook trong `<repo>/.claude/settings.json` được commit → **review kỹ hook của repo lạ** trước khi mở bằng Claude Code.
- Luôn quote biến (`"$file"`), dùng đường dẫn tuyệt đối, và test script bằng tay trước khi gắn.

---

## Xem thêm

- **Cơ chế vận hành tổng quát** (settings, permissions, memory) → [`claude-operation.md`](./claude-operation.md)
- **Hook vs Skill vs CLAUDE.md — chọn đúng chỗ** → [`writing-skills-and-planning.md`](./writing-skills-and-planning.md)
- **Quy trình dùng hằng ngày** → [`claude-code-workflow.md`](./claude-code-workflow.md)
