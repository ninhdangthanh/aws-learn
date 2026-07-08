# Claude Docs — ghi chú cách dùng Claude Code

Bộ note cá nhân về cách vận hành & dùng Claude Code hiệu quả. Chia theo chủ đề:

| File | Nội dung |
|------|----------|
| [`claude-operation.md`](./claude-operation.md) | **Cơ chế nền**: file nào auto-load, `@import`, skills global/project, settings/hooks/permissions, memory, plan mode, thứ tự ưu tiên. |
| [`mcp-guide.md`](./mcp-guide.md) | **MCP** là gì (kết nối tool ngoài), tên tool `mcp__...`, ví dụ gắn MCP server Docker + bảo mật. |
| [`review-and-test-pr.md`](./review-and-test-pr.md) | **Review PR theo ticket Jira** (3 cách: `gh` + dán, Atlassian MCP, GitHub Action) và **test trực tiếp web/React** qua Playwright MCP. |
| [`writing-skills-and-planning.md`](./writing-skills-and-planning.md) | **Cách viết skill** (frontmatter, description, progressive disclosure, test) và **planning** cho task source code. |
| [`claude-code-workflow.md`](./claude-code-workflow.md) | **Quy trình hằng ngày**: thiết lập 1 lần → vòng lặp mỗi task → kỹ năng thực chiến (verify, review, git, debug). |

## Bắt đầu từ đâu?

- Mới tìm hiểu Claude Code → đọc `claude-operation.md` rồi `claude-code-workflow.md`.
- Muốn gắn tool ngoài (Jira, Docker, browser...) → `mcp-guide.md`.
- Việc cụ thể đang cần: review PR / test web → `review-and-test-pr.md`.
- Muốn tự viết skill hoặc plan tốt hơn → `writing-skills-and-planning.md`.
