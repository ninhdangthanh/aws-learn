# Ghi chú cách vận hành của Claude Code

> File tổng hợp cách Claude Code (CLI) hoạt động — dùng để tra cứu. Ghi ngày 2026-07-09.
> Các chủ đề MCP / review PR / viết skills đã tách sang file riêng (xem cuối file).

---

## 1. File nào được auto-load khi mở session mới?

Khi mở một session Claude Code trên terminal, chỉ một số file được **tự động** nạp vào context:

| File | Auto-load? | Ghi chú |
|------|:---------:|---------|
| `CLAUDE.md` (project root) | ✅ | Bộ nhớ chính thức của project |
| `CLAUDE.local.md` | ✅ | Bản local, không commit |
| `~/.claude/CLAUDE.md` (global) | ✅ | Áp dụng cho mọi project |
| File được `@import` trong CLAUDE.md | ✅ | Ví dụ `@RTK.md`, `@AGENTS.md` |
| `CLAUDE.md` ở thư mục con | ✅ | Chỉ load khi đụng file trong thư mục đó |
| Skills (`SKILL.md`) | ⚠️ | **Chỉ load metadata** (`name` + `description`); nội dung đầy đủ chỉ nạp khi skill được gọi |
| **`AGENTS.md`** | ❌ | **KHÔNG** auto-load. Đây là quy ước của tool khác (Codex, Cursor...). Claude Code dùng `CLAUDE.md`. |

**Muốn Claude Code đọc `AGENTS.md`:** thêm dòng `@AGENTS.md` vào `CLAUDE.md` (nhưng đừng để nội dung trùng nhau gây lặp token).

---

## 2. `@import` trong file memory

Trong `CLAUDE.md` có thể trỏ tới file khác bằng cú pháp `@đường-dẫn`:

```markdown
@RTK.md
@AGENTS.md
@docs/conventions.md
```

- Import lồng nhau được (file được import lại import tiếp).
- Đây là cách tách nhỏ config cho gọn.

---

## 3. Skills — global vs project

Skill = một khối kiến thức/quy trình chuyên biệt, Claude tự kích hoạt khi phù hợp hoặc gọi bằng `/tên-skill`.

| Loại | Đường dẫn | Phạm vi |
|------|-----------|---------|
| **Global (personal)** | `~/.claude/skills/<tên>/SKILL.md` | Mọi project trên máy ✅ |
| **Project** | `<repo>/.claude/skills/<tên>/SKILL.md` | Chỉ repo đó; commit git để share team |
| **Plugin** | qua marketplace | Team/tổ chức |

> Hướng dẫn **cách viết** một skill chi tiết → xem [`writing-skills-and-planning.md`](./writing-skills-and-planning.md).

### Cấu trúc một skill

```
~/.claude/skills/
└── my-skill/
    ├── SKILL.md          ← bắt buộc (có frontmatter)
    ├── references/       ← tùy chọn: doc chi tiết, load khi cần
    └── templates/        ← tùy chọn: file mẫu / script
```

### Frontmatter bắt buộc

```markdown
---
name: my-skill
description: Use when ... Examples: "câu người dùng hay hỏi", "...".
---

# Nội dung skill
```

### Điểm mấu chốt: `description`

- Lúc khởi động Claude **chỉ đọc `name` + `description`** để quyết định *khi nào* dùng skill.
- Viết theo **trigger**: bắt đầu bằng "Use when...", kèm ví dụ câu hỏi thực tế.
- Nội dung dài → tách ra `references/` để tiết kiệm token; `SKILL.md` chỉ trỏ tới khi cần.

### Use case: nhiều microservice chung format

Vì các backend nằm ở nhiều repo khác nhau → dùng **global skill** (`~/.claude/skills/`) để mọi repo đều xài chung một bộ quy ước (layout thư mục, cách viết handler, error/logging format...).

---

## 4. Các thành phần "vận hành" khác cần biết

### settings.json & hooks
- `~/.claude/settings.json` (global) và `<repo>/.claude/settings.json` (project) + `settings.local.json`.
- **Hooks**: script tự chạy khi có sự kiện (trước/sau tool, khi stop...). Đây là cách duy nhất để ép hành vi tự động kiểu "mỗi khi X thì làm Y" — vì *harness* chạy hook, không phải Claude tự nhớ.
- Ví dụ trong máy này: hook rewrite `git status` → `rtk git status` để tiết kiệm token.

### Permissions
- Quy định lệnh Bash / tool nào được phép chạy không cần hỏi.
- Cấu hình trong `settings.json` phần `permissions` (allow/deny).

### Slash commands (skills gọi tay)
- Gõ `/tên` để gọi skill/command thủ công, vd `/code-review`, `/init`.

### MCP servers
- Kết nối tool ngoài (vd GitNexus trong repo này cung cấp `impact`, `query`, `context`...).
- Khai báo trong settings; tool xuất hiện dưới tên `mcp__<server>__<tool>`.
- Chi tiết + ví dụ → xem [`mcp-guide.md`](./mcp-guide.md).

### Subagents
- Claude có thể spawn agent con (Explore, Plan, general-purpose...) để làm task phụ trong context riêng.
- Chỉ spawn khi user yêu cầu rõ.

### Memory (bộ nhớ bền vững)
- Claude có thư mục memory riêng ở `~/.claude/projects/<project>/memory/`.
- Mỗi memory = 1 file + 1 dòng index trong `MEMORY.md`.
- Khác với `CLAUDE.md`: memory do Claude tự ghi/tra, `CLAUDE.md` do người viết.

### Plan mode
- Chế độ chỉ đọc để lên kế hoạch trước khi sửa code; thoát bằng khi user duyệt plan.
- Cách planning bài bản → xem [`writing-skills-and-planning.md`](./writing-skills-and-planning.md).

### Context management
- Khi hội thoại quá dài, context cũ được tóm tắt tự động và đưa sang cửa sổ mới → không mất mạch việc.

---

## 5. Thứ tự ưu tiên (tóm tắt)

1. `CLAUDE.md` / settings **global** (`~/.claude/`) — nền chung.
2. `CLAUDE.md` / settings **project** (`<repo>/.claude/`) — ghi đè theo repo.
3. `*.local.md` / `settings.local.json` — ghi đè theo máy cá nhân, không commit.

Instruction trong CLAUDE.md **override** hành vi mặc định của Claude.

---

## Xem thêm (các chủ đề đã tách file)

- **MCP — kết nối tool ngoài + ví dụ Docker** → [`mcp-guide.md`](./mcp-guide.md)
- **Review PR theo Jira & test web/React** → [`review-and-test-pr.md`](./review-and-test-pr.md)
- **Viết skills & planning cho source code** → [`writing-skills-and-planning.md`](./writing-skills-and-planning.md)
- **Quy trình dùng Claude Code hằng ngày** → [`claude-code-workflow.md`](./claude-code-workflow.md)
