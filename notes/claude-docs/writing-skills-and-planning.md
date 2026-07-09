# Viết Skills & Planning cho source code

Hướng dẫn chi tiết 2 việc mà 2 file kia mới nói sơ:
1. **Cách viết một skill** cho đúng (không chỉ "khi nào dùng").
2. **Cách planning** khi cho Claude làm task trên source code.

> Bổ trợ cho `claude-operation.md` (mục 3: cấu trúc skill) và `claude-code-workflow.md` (Pha 1–2: khi nào dùng).

---

## Phần A — Viết Skills

### A1. Skill là gì (nhắc lại nhanh)

Một **gói kiến thức/quy trình** Claude tự kích hoạt khi ngữ cảnh khớp, hoặc gọi tay bằng `/tên-skill`.
Điểm mạnh: **metadata (name + description) luôn được nạp sẵn, nhưng nội dung chỉ nạp khi cần** → không tốn context khi chưa dùng. Đây là cách để có "kiến thức chuyên đề" mà không phình `CLAUDE.md`.

### A2. Cấu trúc thư mục

```
~/.claude/skills/my-skill/        ← global: mọi project trên máy
<repo>/.claude/skills/my-skill/   ← project: chỉ repo này, commit để share team
└── SKILL.md          ← BẮT BUỘC (frontmatter + nội dung)
   references/        ← tùy chọn: doc dài, chỉ đọc khi cần (progressive disclosure)
   templates/         ← tùy chọn: file mẫu, script, snippet
   scripts/           ← tùy chọn: script Claude chạy được
```

Global hay project?
- **Global** — quy ước dùng chung nhiều repo (vd format handler cho các microservice).
- **Project** — kiến thức riêng repo đó (bản đồ module, quy trình release của dự án).

### A3. Frontmatter — phần quan trọng nhất

```markdown
---
name: es-stack-map
description: Use when the user asks how the Elasticsearch layer is wired — indices,
  mappings, ingest flow. Examples: "how does ES indexing work?", "where is the mapping defined?".
---

# Nội dung skill ở đây
```

**`description` quyết định Claude có chọn đúng skill hay không** (lúc khởi động Claude *chỉ* thấy `name` + `description`, chưa thấy nội dung). Quy tắc viết:

- Bắt đầu bằng **"Use when..."** — mô tả *tình huống kích hoạt*, không phải mô tả nội dung.
- Kèm **`Examples:`** là những câu người dùng hay hỏi thật → Claude match tốt hơn.
- Nêu rõ **trigger** (từ khóa, loại task) và cả **khi nào KHÔNG dùng** nếu dễ nhầm với skill khác.
- Ngắn, cụ thể; đừng chung chung kiểu "helps with code".

### A4. Nội dung SKILL.md — nguyên tắc

- **Ngắn gọn, hành động được**: checklist, các bước, lệnh cụ thể — không phải văn xuôi dài.
- **Progressive disclosure**: nội dung dài (schema đầy đủ, ví dụ lớn) tách ra `references/xxx.md`, trong `SKILL.md` chỉ ghi "chi tiết xem `references/xxx.md`" → Claude đọc khi cần, đỡ tốn token.
- **Trỏ tới file/script thật** thay vì chép nội dung: "chạy `scripts/seed.sh`", "template ở `templates/handler.ts`".
- **Một skill = một việc rõ ràng**. Việc to → tách nhiều skill nhỏ, description phân biệt rạch ròi.

### A5. Quy trình viết một skill (thực chiến)

```
1. Xác định trigger  → "khi tôi hỏi/làm X thì cần kiến thức này"
2. Viết description  → Use when... + Examples (câu hỏi thật)
3. Viết thân ngắn    → các bước / checklist / lệnh; cái gì dài → references/
4. Thêm assets       → templates/, scripts/ nếu cần
5. Test              → mở session mới, hỏi câu trong Examples, xem Claude có auto-gọi đúng
6. Chỉnh description  → nếu Claude không tự chọn hoặc chọn nhầm → sửa trigger/Examples
```

> Mẹo test: hỏi một câu **không** chứa nguyên văn tên skill. Nếu Claude vẫn tự kích hoạt → `description` tốt. Nếu phải gõ `/tên` mới chạy → `description` chưa đủ trigger.

### A6. Khi nào KHÔNG nên dùng skill

- Thông tin **luôn cần mọi session** → để `CLAUDE.md`, đừng làm skill.
- Hành vi **"tự động mỗi lần X"** (format sau khi sửa, chặn commit) → **hook** trong `settings.json`, không phải skill.
- Chỉ thị cho **1 task duy nhất** → dùng trực tiếp/Plan mode, không cần lưu skill.

---

## Phần B — Planning cho source code

### B1. Plan mode là per-task, không phải setup

Không phải task nào cũng cần plan. Chọn theo độ phức tạp & rủi ro:

| Loại task | Cách làm |
|---|---|
| Nhỏ, rõ, 1–2 file | Hỏi thẳng → làm luôn, không cần plan |
| Nhiều file / đụng logic quan trọng / chưa chắc cách làm | **Plan mode**: explore → trình plan → **bạn duyệt** → mới code |
| Bug khó, chưa rõ nguyên nhân | Yêu cầu **điều tra root cause trước**, có kết luận rồi mới lên plan sửa |
| Task lớn cần khảo sát rộng | Để Claude spawn agent **Plan** / **Explore** (chỉ khi bạn yêu cầu) |

### B2. Một plan tốt gồm gì

- **Phạm vi & mục tiêu**: làm gì, "done" là như thế nào (đối chiếu acceptance criteria nếu có ticket).
- **File/khu vực sẽ đụng**: liệt kê cụ thể, không mơ hồ.
- **Các bước theo thứ tự**: từng bước nhỏ kiểm chứng được.
- **Rủi ro & phụ thuộc**: đổi cái này thì cái gì gãy (impact analysis), có migration/side-effect ngầm không.
- **Cách verify**: sẽ chạy test/luồng nào để chứng minh xong (xem `claude-code-workflow.md` Pha 3).

### B3. Quy trình chuẩn cho một task code

```
1. Orient    → đọc CLAUDE.md / gọi skill map để nắm chỗ cần đụng
2. Plan      → task lớn: Plan mode, chốt hướng + bạn duyệt trước khi gõ code
3. Implement → sửa từng bước nhỏ, bám quy ước file lân cận (không tự bịa style)
4. Verify    → CHẠY thật (test/build/drive luồng, hoặc browser MCP với web) — không chỉ đọc code
5. Review    → /code-review hoặc /simplify trên diff
6. Commit    → chỉ khi bạn yêu cầu; đang ở main thì tạo nhánh trước
```

### B4. Mẹo giúp plan chất lượng hơn

- **Cho Claude explore trước khi lập plan** — đừng để nó lên plan khi chưa đọc code liên quan.
- **Task đụng hàm dùng nhiều nơi** → hỏi impact analysis trước ("đổi X thì gì phụ thuộc?"). Repo có GitNexus thì dùng skill `gitnexus-impact-analysis`.
- **Chốt hướng trước khi code** — duyệt plan giúp bắt sai lệch sớm, đỡ phải sửa lại nhiều.
- **Task "nhiều khía cạnh" không đồng nghĩa phải spawn subagent** — Claude làm inline được; chỉ spawn khi bạn yêu cầu rõ (tốn kém, mỗi agent khởi động lạnh).

---

## Phần C — Skill vs Plan vs CLAUDE.md vs Hook (chọn đúng chỗ)

| Thứ cần | Bỏ vào đâu |
|---|---|
| Luôn đúng, luôn cần mọi session | `CLAUDE.md` |
| Kiến thức/thủ tục chuyên đề, chỉ cần lúc làm việc đó | **Skill** |
| Hướng giải cho 1 task cụ thể (dùng xong bỏ) | **Plan mode** (ephemeral) |
| Điều muốn Claude nhớ lâu (sở thích, quyết định) | Memory |
| Hành vi tự động "mỗi lần X thì Y" | **Hook** trong `settings.json` (không phải trí nhớ/skill) → [`hooks-guide.md`](./hooks-guide.md) |
