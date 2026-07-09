# MCP — kết nối tool/hệ thống ngoài vào Claude

> Tách từ `claude-operation.md`. Giải thích khái niệm MCP + ví dụ gắn một MCP server thật (Docker).
> Ứng dụng MCP cho review PR / test web xem `review-and-test-pr.md`.

---

## 1. MCP là gì?

**MCP = Model Context Protocol** — một "chuẩn cắm" (giống cổng USB) để Claude kết nối với **hệ thống bên ngoài**. Thay vì chỉ đọc/ghi file trong máy, MCP cho Claude:

- **Đọc dữ liệu** từ nơi khác (Jira ticket, DB, GitHub, Google Drive...).
- **Thực hiện hành động** ở nơi khác (tạo issue, query DB, điều khiển Docker...).

Mỗi kết nối = một **MCP server**. Claude Code là **MCP client**, "cắm" vào các server đó.

### Có thể gắn gì? (ví dụ)

| MCP server | Cho Claude khả năng gì |
|-----------|------------------------|
| Jira/Atlassian | Đọc ticket, tạo/cập nhật issue, comment |
| GitHub | Đọc PR, tạo issue, review code |
| **Docker** | Liệt kê container, xem logs, restart, chạy lệnh |
| PostgreSQL/DB | Query dữ liệu trực tiếp |
| Google Drive | Đọc/tìm file |
| Playwright / Chrome | Mở & điều khiển trình duyệt (test web) |
| GitNexus | Phân tích code graph (repo này đang dùng) |

### Quy ước tên tool

Tool của MCP server xuất hiện dưới dạng: `mcp__<tên-server>__<tên-tool>`

Ví dụ đang chạy trong repo này:
- `mcp__gitnexus__impact`, `mcp__gitnexus__query` → server **GitNexus**
- `mcp__claude_ai_Google_Drive__search_files` → server **Google Drive**

### Lưu ý về Docker (2 vai trò khác nhau)

- Docker **là công cụ để chạy** một MCP server (nhiều server đóng gói dạng Docker image).
- Docker cũng có thể **là đối tượng** mà một "Docker MCP server" điều khiển.

---

## 2. Ví dụ thực tế: gắn MCP server quản lý Docker

Mục tiêu: cho Claude xem/điều khiển các container Docker của bạn (list, logs, start/stop).

### Bước 1 — Khai báo server

Thêm vào `~/.claude/settings.json` (global, dùng cho mọi project) hoặc `.mcp.json` ở root repo (chỉ project này, commit được để share team):

```json
{
  "mcpServers": {
    "docker": {
      "command": "npx",
      "args": ["-y", "mcp-server-docker"],
      "env": {
        "DOCKER_HOST": "unix:///var/run/docker.sock"
      }
    }
  }
}
```

> - `command`/`args`: lệnh để khởi động server (ở đây chạy qua `npx`; nhiều server khác chạy bằng `docker run ...`).
> - `DOCKER_HOST`: trỏ tới Docker daemon. Mặc định trên macOS/Linux là `unix:///var/run/docker.sock`.
> - Tên server tùy chọn (`docker`) → sẽ thành prefix `mcp__docker__...`.

### Bước 2 — Mở lại session

MCP server chỉ được nạp khi khởi động session. Thoát và mở lại Claude Code.
Kiểm tra bằng lệnh trong Claude Code:

```
/mcp
```

→ liệt kê các server đang kết nối và trạng thái.

### Bước 3 — Dùng

Giờ bạn có thể yêu cầu bằng ngôn ngữ tự nhiên:

- "Liệt kê các container đang chạy"
- "Xem 100 dòng log cuối của container `order-pos`"
- "Restart container `postgres`"

Claude sẽ tự gọi tool `mcp__docker__list_containers`, `mcp__docker__get_logs`, v.v.

### Bảo mật cần lưu ý

- MCP server Docker có quyền điều khiển daemon → **quyền rất mạnh** (tương đương quyền chạy docker trên máy). Chỉ gắn server bạn tin tưởng.
- Có thể giới hạn bằng **permissions** trong `settings.json`: chỉ allow các tool đọc (list, logs), còn hành động phá hủy (remove, prune) thì để hỏi xác nhận.
- Token/secret (API key Jira, DB password...) đặt trong `env`, đừng commit file chứa secret — dùng `settings.local.json` hoặc biến môi trường.

> Ghi chú: tên package `mcp-server-docker` ở trên chỉ là minh họa. Khi triển khai thật, chọn một Docker MCP server cụ thể (kiểm tra README của nó để biết `command`/`args`/`env` chính xác).

## 3. Ví dụ thực tế: gắn trình duyệt Firefox (Playwright MCP)

Mục tiêu: cho Claude mở trình duyệt, search, click, đọc nội dung trang (collect data).

### Cài đặt (đã làm 2026-07-09)

```bash
# Đăng ký server ở user scope (dùng cho mọi project)
claude mcp add playwright -s user -- npx -y @playwright/mcp@latest --browser firefox

# Tải Firefox build của Playwright (nếu chưa có trong ~/Library/Caches/ms-playwright/)
npx playwright install firefox
```

Restart session → `/mcp` để kiểm tra. Tool xuất hiện dạng `mcp__playwright__browser_navigate`, `browser_click`, `browser_type`, `browser_snapshot`...

### Hiện cửa sổ hay chạy ngầm?

- **Mặc định: headed** — cửa sổ Firefox mở lên, bạn nhìn thấy Claude thao tác trực tiếp.
- Muốn **chạy ngầm (headless)**: thêm flag `--headless` vào args của server.
- Đây là **Firefox build riêng của Playwright**, không phải Firefox bạn cài — không dùng profile/bookmark/login cá nhân. Playwright MCP tự giữ một profile riêng nên cookie/login trong đó vẫn persist giữa các session (thêm `--isolated` nếu muốn sạch mỗi lần).

### Flow khi bảo Claude "search X"

1. `browser_navigate` → mở trang search (Google/DuckDuckGo).
2. `browser_snapshot` → đọc accessibility tree của trang kết quả (không phải screenshot, rất ít token).
3. Chọn kết quả uy tín → `browser_click` vào link.
4. `browser_snapshot`/`browser_evaluate` trên trang đích → trích xuất dữ liệu, tổng hợp trả lời.

> Lưu ý: nếu chỉ cần "search và tóm tắt", tool `WebSearch`/`WebFetch` có sẵn nhanh và rẻ hơn nhiều. Browser MCP đáng dùng khi cần **tương tác** (login, form, trang render bằng JS, test UI).

---

### Cách thêm MCP nhanh bằng CLI

Ngoài sửa tay `settings.json`, có thể dùng lệnh:

```bash
claude mcp add <tên> <command> [args...]           # server chạy local (stdio)
claude mcp add --transport sse <tên> <url>         # server remote (SSE), vd Atlassian cloud
```
