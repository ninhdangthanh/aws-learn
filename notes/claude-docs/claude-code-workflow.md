# Dùng Claude Code trong dự án — thứ tự & cách tổ chức hợp lý

Ghi chú về cách bố trí các cơ chế của Claude Code (CLAUDE.md, skills, plan mode, memory, subagents)
sao cho hợp lý. Chia làm 2 pha: **thiết lập 1 lần** và **mỗi task lặp lại**.

## Pha 1 — Thiết lập nền (làm 1 lần, theo thứ tự)

### 1. `CLAUDE.md` trước tiên — nền tảng
- File **duy nhất được auto-load mỗi session**. Đặt kiến trúc tổng quan, quy ước, cạm bẫy, lệnh hay dùng.
- Cách nhanh: chạy `/init` để Claude sinh bản nháp rồi sửa.
- Nguyên tắc: **ngắn gọn**, chỉ chứa thứ không suy ra được từ code. Càng dài càng loãng.

### 2. Skills — cho kiến thức/quy trình lặp lại
- Dùng khi có **domain knowledge hoặc thủ tục** muốn tái sử dụng (vd 6 skill `es-*`: bản đồ, verify, troubleshoot).
- Metadata (name + description) được auto-load; **nội dung chỉ nạp khi được gọi** → không tốn context khi chưa cần.
  Đây là điểm hơn hẳn việc nhồi hết vào CLAUDE.md.
- Viết `description` rõ "**dùng khi hỏi X**" → Claude tự chọn đúng lúc.

### 3. Permissions / settings — giảm ma sát
- Allowlist các lệnh đọc an toàn (`git status`, `ls`…) để bớt bị hỏi quyền.
- Skill `/fewer-permission-prompts` quét transcript và đề xuất allowlist.

## Pha 2 — Mỗi task (vòng lặp)

**Plan mode KHÔNG phải thứ setup — nó là per-task.** Quyết định theo độ phức tạp:

| Loại task | Cách làm |
|---|---|
| Rõ ràng, nhỏ, 1-2 file | Hỏi thẳng → Claude làm luôn |
| Phức tạp / nhiều file / rủi ro / chưa chắc cách tiếp cận | **Plan mode**: explore → trình plan → **duyệt** → mới execute |
| Cần orient trước khi đụng code | Để Claude gọi **skill** map, hoặc gõ `/es-stack-map` |

## Thứ tự ưu tiên / "sức nặng" khi Claude làm việc

```
CLAUDE.md   → luôn có (mọi session)          → quy ước + định hướng
   ↓
Skills      → nạp metadata sẵn, gọi khi khớp → kiến thức/thủ tục tái dùng
   ↓
Plan mode   → per-task, cho việc lớn/rủi ro  → chốt hướng trước khi code
   ↓
Memory      → fact bền, recall theo ngữ cảnh → điều bạn dạy Claude qua thời gian
   ↓
Subagents   → chỉ khi bạn yêu cầu            → việc nặng/song song, tốn kém
```

## Quy tắc chọn "nên bỏ vào đâu"

- Thông tin **luôn đúng, luôn cần** → `CLAUDE.md`.
- **Thủ tục/kiến thức chuyên đề, chỉ cần lúc làm việc đó** → `skill`.
- **Hướng giải quyết cho 1 task cụ thể** → `plan mode` (ephemeral, không lưu).
- **Điều muốn Claude nhớ lâu dài** (sở thích, quyết định) → memory.

## Chú ý quan trọng (dễ hiểu lầm)

- **README không khiến Claude "đọc được" skills.** Skill metadata do harness auto-load, độc lập với README.
  Mục skills trong README thuần cho **người** đọc repo.
- **README không auto-load đầu session** — chỉ `CLAUDE.md` mới auto-load. Đưa gì lên đầu README cũng không đổi điều này.
- Muốn Claude **ưu tiên gọi skill** thay vì tự explore → viết chỉ dẫn đó trong `CLAUDE.md`.
