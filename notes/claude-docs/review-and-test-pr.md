# Review PR theo Jira & test trực tiếp web/React

> Tách từ `claude-operation.md`. Khái niệm MCP nền tảng xem `mcp-guide.md`.
> Hai việc nối tiếp nhau: **(1) review code PR đối chiếu ticket Jira** → **(2) mở app lên test thật**.

---

## 1. Cho Claude review GitHub PR dựa trên ticket Jira

Mục tiêu: Claude đọc **yêu cầu trong ticket Jira** + **code trong Pull Request**, rồi đánh giá xem PR đã đáp ứng đủ yêu cầu chưa, thiếu case nào, rủi ro khi merge.

### Ý tưởng cốt lõi

Claude cần tiếp cận **2 nguồn dữ liệu**:

1. **Nội dung PR** (diff + mô tả) → lấy qua `gh` CLI (có sẵn) hoặc GitHub MCP.
2. **Nội dung ticket Jira** (mô tả + acceptance criteria) → dán tay, hoặc để Claude tự đọc qua **Atlassian MCP** (khái niệm MCP xem `mcp-guide.md`).

Có ticket Jira thì việc review mới có "thước đo" để đối chiếu — thay vì chỉ soi bug thuần code.

### Cách 1 — Đơn giản nhất: `gh` CLI + dán ticket (không cần cấu hình gì)

Claude Code đã có sẵn `gh`. Chỉ cần dán yêu cầu Jira vào prompt:

```
Review PR #123 dựa trên yêu cầu Jira ticket PROJ-456 dưới đây:

<dán mô tả + acceptance criteria từ Jira>

Kiểm tra: code có đáp ứng đủ acceptance criteria không, thiếu case nào,
rủi ro khi merge, và gợi ý sửa nếu có.
```

Claude sẽ tự chạy `gh pr view 123` / `gh pr diff 123` để lấy code rồi đối chiếu. **Nên bắt đầu bằng cách này** để thấy chất lượng review ngay.

### Cách 2 — Tự động: kết nối Jira qua Atlassian MCP (khỏi copy-paste)

Cài Atlassian MCP server (remote, dùng cho **Jira Cloud**):

```bash
claude mcp add --transport sse atlassian https://mcp.atlassian.com/v1/sse
```

Sau khi OAuth xong, chỉ cần nói:

```
Đọc Jira ticket PROJ-456 và review PR #123 xem đã đáp ứng yêu cầu chưa.
```

Claude tự gọi tool `mcp__atlassian__...` lấy ticket + tự lấy diff PR. Có thể thêm cả **GitHub MCP** nếu muốn thay `gh`.

> ⚠️ **Jira Server / Data Center** (self-hosted) không dùng remote MCP cloud ở trên được → phải chạy một MCP server local trỏ tới URL Jira nội bộ + API token (khai báo `command`/`args`/`env` giống mẫu Docker ở `mcp-guide.md`). Kiểm tra loại Jira mình đang dùng trước khi chọn.

### Cách 3 — Hoàn toàn tự động qua GitHub Action (mỗi PR mới → Claude tự review)

Dùng `anthropics/claude-code-action` chạy trong CI: khi có PR, Claude đọc diff, lấy ticket Jira (thường suy ra ticket key từ tên branch / title PR), rồi **comment thẳng vào PR**. Phù hợp khi team review thường xuyên, muốn tự động hoàn toàn.

### Skill/lệnh sẵn có liên quan

- `/review <PR#>` — review một GitHub PR (bổ sung context Jira vào prompt).
- `/code-review` — soi diff của branch hiện tại (local, chưa cần push).
- `/code-review ultra` — review đa-agent trên cloud cho PR lớn.

### Lộ trình gợi ý

1. Thử **Cách 1** ngay để cảm nhận chất lượng review.
2. Nếu review thường xuyên → nâng lên **Cách 2 (Atlassian MCP)** để khỏi dán tay.
3. Muốn tự động hẳn cho cả team → dựng **Cách 3 (GitHub Action)**.

> Cần xác định trước: **Jira Cloud hay Server/Data Center** (quyết định cách kết nối ở Cách 2), và có muốn dùng **GitHub MCP** thay `gh` không.

---

## 2. Cho Claude test trực tiếp trang web/React của PR (browser automation)

Sau khi review code PR, thường muốn **mở trang lên bấm thử** theo acceptance criteria — không chỉ đọc code. Claude làm được nhờ **browser automation qua MCP**.

### "Plugin" để mở/điều khiển trình duyệt

| MCP server | Cho Claude khả năng gì |
|-----------|------------------------|
| **Playwright MCP** (`@playwright/mcp`) | Mở browser thật, click, điền form, chụp screenshot, đọc DOM + console log. Phổ biến nhất. |
| **Chrome DevTools MCP** | Gắn vào Chrome, xem network / console / performance. |

Cài một lần (khái niệm MCP xem `mcp-guide.md`), ví dụ Playwright:

```bash
claude mcp add playwright npx @playwright/mcp@latest
```

→ tool xuất hiện dưới tên `mcp__playwright__...` (mở trang, click, screenshot).

### Skill sẵn có phối hợp

- **`/run`** — khởi động app của repo (tự nhận diện loại project: web/dev server, CLI, Electron...).
- **`/verify`** — drive luồng thật rồi **quan sát hành vi**, không dừng ở "typecheck pass".

### Quy trình test một PR React

```
1. gh pr checkout 123          → lấy code PR về máy
2. /run  hoặc  npm run dev     → dựng dev server (vd localhost:5173)
3. Claude gọi mcp__playwright__ → mở trang, thao tác theo acceptance criteria
4. Chụp screenshot / đọc console → show cho bạn thấy tận mắt + báo lỗi nếu có
```

Kết quả: Claude không chỉ review code mà **mở trang, bấm thử đúng kịch bản trong Jira, và trả về screenshot** để bạn kiểm chứng trực quan.

> Lưu ý: browser MCP điều khiển được trình duyệt thật → chỉ trỏ vào app dev/local của bạn, cẩn trọng khi cho thao tác trên trang có dữ liệu thật/đăng nhập.
