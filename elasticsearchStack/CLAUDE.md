# CLAUDE.md — elasticsearchStack

Project học Elasticsearch: **Postgres = source of truth**, **ES = secondary store cho search**,
đồng bộ qua **outbox pattern** (ghi `products` + `outbox` cùng transaction → worker sync ES qua alias).
Stack: Docker (ES 8.14 + Kibana + Postgres 16) → Backend Go/Gin (`:8090`) → Frontend React/Vite (`:5173`).
Tổng quan đầy đủ: xem [README.md](README.md).

## Ưu tiên dùng Skills để định hướng

Trước khi tự explore code, **gọi skill phù hợp trước** để orient — chúng là bản đồ có sẵn của project:

- `es-stack-map` — tổng hệ thống: stack/service, kiến trúc sync, port, cách chạy.
- `es-backend-map` — backend Go: package, luồng outbox worker → ES, endpoint, env.
- `es-frontend-map` — frontend React: component, `api.ts`, 2 tab + StatusBar.
- `es-query-guide` — viết Query DSL & aggregation trên index `products`.
- `es-data-verify` — verify dữ liệu ES đúng chưa, đối soát PG↔ES, mapping, outbox.
- `es-troubleshoot` — gỡ lỗi vận hành: docker không lên, port bận, reset stack.

Các skill là **orientation/tra cứu**, không phải để thêm feature. Khi câu hỏi khớp mô tả một skill,
gọi nó trước rồi mới đọc file cụ thể.

## Ghi chú

- App luôn đọc/ghi ES **qua alias** `products` (trỏ `products_v1`), không trỏ thẳng index thật.
- Backend "chiếm" tên `products` làm alias — muốn chơi lại Phase 2-4 thì chạy `./scripts/seed-products.sh` khi backend chưa chạy.
- `WRITE_MODE=dual` là **bài học phản diện** (ghi thẳng ES, ES down → lệch vĩnh viễn), không phải mode dùng thật.
