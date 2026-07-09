# Dùng Claude Code trong dự án — thứ tự & cách tổ chức hợp lý

Ghi chú về cách bố trí các cơ chế của Claude Code (CLAUDE.md, skills, plan mode, memory, subagents)
sao cho hợp lý, cộng với **quy trình thực chiến hằng ngày của một developer**: từ nhận task →
lên kế hoạch → code → verify → review → commit.

Chia làm 3 phần: **thiết lập 1 lần**, **vòng lặp mỗi task**, và **kỹ năng thực chiến khi code**.

---

## Pha 1 — Thiết lập nền (làm 1 lần, theo thứ tự)

### 1. `CLAUDE.md` trước tiên — nền tảng
- File **duy nhất được auto-load mỗi session**. Đặt kiến trúc tổng quan, quy ước, cạm bẫy, lệnh hay dùng.
- Cách nhanh: chạy `/init` để Claude sinh bản nháp rồi sửa.
- Nguyên tắc: **ngắn gọn**, chỉ chứa thứ không suy ra được từ code. Càng dài càng loãng.
- Nên có trong CLAUDE.md của một dự án code:
  - **Kiến trúc 1 đoạn**: các service, luồng dữ liệu chính, "source of truth" nằm ở đâu.
  - **Lệnh hay dùng**: build, run, test, lint, migrate (Claude sẽ dùng đúng thay vì đoán).
  - **Quy ước code**: naming, cấu trúc thư mục, pattern bắt buộc (vd "luôn ghi qua alias").
  - **Cạm bẫy**: những chỗ dễ sai, side-effect ngầm, thứ "đừng đụng vào".
  - **Chỉ dẫn hành vi**: vd "ưu tiên gọi skill map trước khi explore", "chỉ commit khi tôi yêu cầu".

### 2. Skills — cho kiến thức/quy trình lặp lại
- Dùng khi có **domain knowledge hoặc thủ tục** muốn tái sử dụng (vd 6 skill `es-*`: bản đồ, verify, troubleshoot).
- Metadata (name + description) được auto-load; **nội dung chỉ nạp khi được gọi** → không tốn context khi chưa cần.
  Đây là điểm hơn hẳn việc nhồi hết vào CLAUDE.md.
- Viết `description` rõ "**dùng khi hỏi X**" → Claude tự chọn đúng lúc.
- Với developer, skill hợp lý cho: bản đồ codebase theo module, checklist verify, quy trình release,
  hướng dẫn viết query/migration, cách debug một hệ con phức tạp.

### 3. Permissions / settings — giảm ma sát
- Allowlist các lệnh đọc an toàn (`git status`, `ls`, `git diff`, `git log`…) để bớt bị hỏi quyền.
- Skill `/fewer-permission-prompts` quét transcript và đề xuất allowlist.
- Cân nhắc allowlist thêm: lệnh test/build/lint hay chạy, `gh pr view`, `rg`/`grep`.
- **Đừng allowlist** lệnh phá hủy (`rm -rf`, `git push --force`, `git reset --hard`, drop DB) — giữ để phải xác nhận.

### 4. Hooks — tự động hóa hành vi lặp
- Muốn "mỗi lần X thì làm Y" (vd chạy formatter sau khi sửa file, chặn commit vào `main`) → dùng **hook** trong
  `settings.json`, không phải nhờ Claude nhớ. Harness chạy hook, không phải Claude → mới đảm bảo luôn xảy ra.
- Skill `/update-config` giúp cấu hình hook/permission/env.

---

## Pha 2 — Mỗi task (vòng lặp)

**Plan mode KHÔNG phải thứ setup — nó là per-task.** Quyết định theo độ phức tạp:

| Loại task | Cách làm |
|---|---|
| Rõ ràng, nhỏ, 1-2 file | Hỏi thẳng → Claude làm luôn |
| Phức tạp / nhiều file / rủi ro / chưa chắc cách tiếp cận | **Plan mode**: explore → trình plan → **duyệt** → mới execute |
| Cần orient trước khi đụng code | Để Claude gọi **skill** map, hoặc gõ `/es-stack-map` |
| Bug khó / không rõ nguyên nhân | Yêu cầu **điều tra trước, sửa sau**: tìm root cause → giải thích → mới vá |

### Vòng lặp chuẩn cho một task code

```
1. Orient   → gọi skill map / đọc CLAUDE.md để nắm chỗ cần đụng
2. Plan     → task lớn thì Plan mode, chốt hướng trước khi gõ code
3. Implement→ sửa code theo từng bước nhỏ, bám quy ước sẵn có
4. Verify   → CHẠY thử thật (test/build/drive luồng), không chỉ đọc code
5. Review   → /code-review hoặc /simplify trên diff trước khi commit
6. Commit   → chỉ khi bạn yêu cầu; branch nếu đang ở main
```

---

## Pha 3 — Kỹ năng thực chiến khi code

### Verify: luôn chạy thật, đừng tin "nhìn code thấy đúng"
- Với thay đổi có runtime, dùng skill `/verify` hoặc `/run` để **drive luồng thật** rồi quan sát hành vi,
  không dừng ở "typecheck pass".
- Báo cáo trung thực: test fail thì nói fail kèm output; bước nào bỏ qua thì nói rõ. Đừng "làm tròn" thành xong.
- Thứ tự tin cậy: *chạy được luồng thật* > *test pass* > *typecheck/lint pass* > *đọc code thấy hợp lý*.

### Code review trước khi commit
- `/code-review` — soi diff hiện tại tìm **bug đúng/sai** + gợi ý dọn dẹp. Chọn effort:
  `low/medium` (ít, chắc), `high→max` (rộng hơn, có thể có cái chưa chắc).
- `/simplify` — chỉ dọn reuse/đơn giản hóa/hiệu năng, **không** săn bug.
- `/code-review ultra` — review đa-agent trên cloud cho nhánh/PR lớn (do bạn tự kích hoạt).
- Quy tắc: task nhỏ dùng `/simplify`; task đụng logic quan trọng dùng `/code-review` rồi mới commit.

### Git & commit — kỷ luật
- **Chỉ commit/push khi bạn yêu cầu.** Đang ở nhánh mặc định (`main`) thì **tạo nhánh trước**.
- Commit message rõ *tại sao*, không chỉ *cái gì*. Kết thúc bằng dòng `Co-Authored-By` nếu quy ước dự án cần.
- Trước khi xóa/ghi đè file: nhìn vào nội dung thật; nếu khác mô tả hoặc không phải bạn tạo → hỏi lại.
- Dùng `gh` cho thao tác GitHub (PR, issue). PR body nêu tóm tắt thay đổi + cách test.

### Debug — điều tra trước, vá sau
- Yêu cầu Claude **tái hiện lỗi / tìm root cause** trước khi sửa; đừng vá triệu chứng.
- Với dự án có GitNexus: các skill `gitnexus-debugging`, `gitnexus-exploring`, `gitnexus-impact-analysis`
  giúp trace lỗi, hiểu luồng, biết "đổi cái này thì gãy đâu".
- Trước khi sửa hàm dùng nhiều nơi: hỏi **impact analysis** ("đổi X thì cái gì phụ thuộc?").

### Subagents & parallel — chỉ khi cần
- **Chỉ spawn subagent khi bạn yêu cầu.** Mỗi agent khởi động "lạnh", tự dựng lại context → tốn kém.
- Dùng cho: khảo sát rộng nhiều file (`Explore`), lập kế hoạch kiến trúc (`Plan`),
  việc nặng chạy song song/nền.
- Task "nhiều khía cạnh / kỹ lưỡng" **không** đồng nghĩa phải spawn — Claude tự làm inline được.

### Làm nhiều việc song song trong 1 lượt
- Các thao tác **độc lập** (đọc nhiều file, chạy nhiều lệnh không phụ thuộc nhau) nên gộp vào **một lượt**
  để chạy song song → nhanh hơn. Chỉ tuần tự khi bước sau cần kết quả bước trước.

---

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

---

## Quy tắc chọn "nên bỏ vào đâu"

- Thông tin **luôn đúng, luôn cần** → `CLAUDE.md`.
- **Thủ tục/kiến thức chuyên đề, chỉ cần lúc làm việc đó** → `skill`.
- **Hướng giải quyết cho 1 task cụ thể** → `plan mode` (ephemeral, không lưu).
- **Điều muốn Claude nhớ lâu dài** (sở thích, quyết định, phản hồi cách làm việc) → memory.
- **Hành vi tự động "mỗi lần X"** (format, chặn commit, chạy hook) → `settings.json` / hook, không nhờ trí nhớ.

---

## Chú ý quan trọng (dễ hiểu lầm)

- **README không khiến Claude "đọc được" skills.** Skill metadata do harness auto-load, độc lập với README.
  Mục skills trong README thuần cho **người** đọc repo.
- **README không auto-load đầu session** — chỉ `CLAUDE.md` mới auto-load. Đưa gì lên đầu README cũng không đổi điều này.
- Muốn Claude **ưu tiên gọi skill** thay vì tự explore → viết chỉ dẫn đó trong `CLAUDE.md`.
- **Memory phản ánh lúc được ghi**, không phải hiện tại — nếu nó nhắc tên file/hàm/flag, phải kiểm tra còn tồn tại không trước khi tin.
- **Nhờ Claude "nhớ mỗi lần làm X"** không thực hiện được bằng memory — đó là việc của hook trong settings.

---

## Checklist nhanh cho mỗi task

- [ ] Đã orient (skill map / CLAUDE.md) trước khi đụng code?
- [ ] Task lớn/rủi ro → đã qua Plan mode và **duyệt plan**?
- [ ] Code bám quy ước sẵn có (đọc file lân cận, không tự bịa style)?
- [ ] Đã **verify chạy thật**, không chỉ typecheck?
- [ ] Đã `/code-review` hoặc `/simplify` trên diff?
- [ ] Commit đúng lúc (bạn yêu cầu), đúng nhánh (không phải `main`), message rõ *tại sao*?
