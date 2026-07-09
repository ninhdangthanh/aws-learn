---
name: es-stack-map
description: >
  Bản đồ hệ thống của project notes/elasticsearchStack — hệ thống gồm những gì,
  stack/service nào, kiến trúc sync Postgres↔Elasticsearch, và cách chạy từng phần
  (Docker, backend Go, frontend React, script demo). Dùng để orient/định hướng khi
  hỏi "project này có gì", "chạy sao", "stack gì", "kiến trúc thế nào", "port nào",
  "service nào", trước khi đụng vào code. KHÔNG phải để thêm feature mới.
---

# Elasticsearch Stack — Bản đồ hệ thống

Project học ES thực chiến: **Postgres = source of truth**, **Elasticsearch = secondary store cho search**.
App luôn đọc/ghi ES **qua alias** `products`. Data đồng bộ PG→ES qua **outbox worker** (mặc định).

> Đây là skill **orientation** (biết hệ thống có gì, chạy sao). Không dùng để implement feature.
> Chi tiết đầy đủ: `README.md`. Kế hoạch theo phase: `elasticsearch-implement-plan.md`.

**Skill con (đi sâu từng phần):**
- `es-backend-map` — backend Go/Gin: package, endpoint, outbox worker, env.
- `es-frontend-map` — frontend React/Vite: component, api.ts, 2 tab + StatusBar.
- `es-data-verify` — kiểm tra dữ liệu ES có đúng chưa (reconcile PG↔ES, mapping, drift).
- `es-query-guide` — viết/chạy Query DSL & aggregation trên index products.
- `es-troubleshoot` — gỡ lỗi vận hành (stack không lên, port, migrate, reset, chơi lại phase).

## Stack & service (cổng)

| Thành phần | Công nghệ | Cổng | Ghi chú |
|---|---|---|---|
| Elasticsearch | ES 8.14 (Docker) | `9200` | `single-node`, security **tắt** (chỉ local) |
| Kibana | Kibana 8.14 (Docker) | `5601` | Dev Tools + Dashboard |
| Postgres | postgres:16 (Docker) | `5433`→5432 | user/pass/db = `app`/`app`/`shop`. Source of truth |
| Backend | Go + Gin (`backend/`) | `8090` | REST: `/search` + CRUD + `/admin/*` |
| Frontend | React + Vite (`frontend/`) | `5173` | Tab Search (ES) + Admin CRUD (PG) + status bar |

## Layout thư mục

```
docker-compose.yml   ES + Kibana + Postgres (local, security tắt)
queries/             .http theo phase (Query DSL, aggregation) — chạy bằng curl / Kibana Dev Tools
scripts/             seed / verify / demo (phase5-demo.sh, phase5-stale-test.sh, seed-products.sh, ...)
backend/             Go + Gin: handler/ (search, admin), outbox/worker.go, esclient/, store/, config/
frontend/            React + Vite: SearchPage / AdminPage / StatusBar / api.ts
elasticsearch-implement-plan.md   kế hoạch chi tiết theo phase
```

## Kiến trúc sync PG→ES (điểm cốt lõi)

- Ghi (`POST/PUT/DELETE /products`) → ghi bảng `products` **+ bảng `outbox` trong cùng transaction**.
- **Worker** (`internal/outbox/worker.go`) poll outbox (`FOR UPDATE SKIP LOCKED`), index ES với
  **external version** (`updated_at` epoch) → chống stale write; ES trả **409** = bỏ qua an toàn. At-least-once + idempotent (`_id` = product id).
- Đọc search (`GET /search`) → **chỉ query ES qua alias**, không đụng SQL.
- Đọc admin list (`GET /products`) → query **Postgres** (phản ánh ngay, không chờ worker).
- `WRITE_MODE` (env): `outbox` (mặc định, đáng tin) vs `dual` (demo lỗi dual-write — tắt ES là lệch vĩnh viễn).

## Cách chạy

**1. Hạ tầng Docker** (từ `notes/elasticsearchStack/`):
```bash
docker compose up -d elasticsearch kibana postgres
./scripts/verify.sh          # kiểm tra ES sống
```

**2. Backend** (cần Postgres chạy):
```bash
cd backend && cp .env.example .env    # default đã khớp compose
go run .                              # :8090 — tự migrate + tạo alias + chạy worker
```

**3. Frontend** (cần backend chạy):
```bash
cd frontend && cp .env.example .env   # VITE_API_BASE=http://localhost:8090
npm install && npm run dev            # http://localhost:5173
```

**4. Demo end-to-end / học từng phase**:
```bash
./scripts/phase5-demo.sh         # outbox sync + drift recovery + backfill + reindex
./scripts/phase5-stale-test.sh   # test external-version chống stale (5.4b)
./scripts/seed-products.sh       # seed products cho Phase 2-4 (chạy khi backend CHƯA chạy)
```
> Backend "chiếm" tên `products` làm alias — muốn chơi lại Phase 2-4 (index thường) thì seed khi backend chưa chạy.

## API backend (tham chiếu nhanh)

- `GET /search?q=&category=&status=&brand=&in_stock=&min_price=&max_price=&size=&from=` — query ES.
- `POST /products`, `PUT /products/:id`, `DELETE /products/:id` — ghi Postgres (+ outbox cùng transaction).
- `GET /products?limit=` — list từ Postgres (source of truth).
- `POST /admin/backfill` — nạp toàn bộ PG→ES bằng `_bulk` (idempotent).
- `GET /admin/reconcile` — đối soát nông: COUNT(*) PG vs `_count` ES + outbox pending.
- `GET /admin/reconcile/deep?fix=true` — đối soát sâu theo `updated_at`; `fix=true` re-index từ DB.
- `POST /admin/reindex` — reindex zero-downtime, swap alias atomic.

## Tiến độ theo phase

- [x] P1 Docker ES+Kibana · [x] P2 index/mapping/CRUD · [x] P3 Query DSL · [x] P4 Aggregation+Kibana
- [x] P5 Backend + sync (outbox/dual, reconcile, reindex) · [ ] P6 Search API + React UI · [ ] P7 tổng kết

## Gotchas đã học

- ES 8.x **cấm fielddata trên `_id`** → tie-breaker sort dùng field keyword riêng (`sku`), không dùng `_id`.
- `text` (analyzed, full-text `match`) vs `keyword` (nguyên văn, `term`/sort/aggregate).
- Postgres map cổng **5433** (tránh đụng 5432 máy local).
- Chế độ `dual`: tắt ES giữa chừng → lệch **vĩnh viễn**, không hồi phục (đây là bài học, không phải bug).
