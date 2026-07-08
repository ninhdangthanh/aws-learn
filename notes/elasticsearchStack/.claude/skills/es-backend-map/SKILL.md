---
name: es-backend-map
description: >
  Bản đồ backend Go (Gin) của project elasticsearchStack — cấu trúc package, luồng
  ghi Postgres + outbox worker → Elasticsearch, danh sách endpoint, env config, cách
  chạy/migrate. Dùng khi hỏi về BE: "backend có gì", "endpoint nào", "worker chạy sao",
  "config/env", "package nào làm gì", "chạy backend thế nào". Orientation, không phải thêm feature.
---

# Backend (Go + Gin) — Bản đồ

Module `es-sync-backend`, cổng **:8090**. **Postgres = source of truth**, ES = secondary store cho search (đọc/ghi **qua alias** `products`).

## Cấu trúc package (`backend/internal/`)

| Package | File | Vai trò |
|---|---|---|
| `config` | `config.go` | Load env (có default khớp docker-compose): `PORT`, `DATABASE_URL`, `ES_URL`, `ES_ALIAS`, `WRITE_MODE`, `WORKER_POLL`, `RUN_WORKER`, `CORS_ORIGIN` |
| `store` | `store.go`, `outbox.go` | Postgres (pgxpool). CRUD `products` **+ ghi `outbox` cùng transaction**; keyset scan; count/reconcile helpers |
| `esclient` | `esclient.go`, `mapping.go`, `mget.go` | HTTP client ES: index/delete doc (external version), `_bulk`, `_count`, `EnsureIndexAlias`, `ReindexSwap` (alias atomic) |
| `outbox` | `worker.go` | Poll outbox `FOR UPDATE SKIP LOCKED` → index ES với external version, retry (at-least-once, idempotent) |
| `handler` | `handler.go`, `search.go`, `admin.go` | REST: `/search` (đọc ES), CRUD (ghi PG), `/admin/*` |

`main.go`: load env → connect PG → `Migrate(migrations/001_init.sql)` → `es.EnsureIndexAlias` → (nếu `RUN_WORKER` & mode≠dual) chạy `outbox.Worker` goroutine → Gin + CORS → serve, graceful shutdown.

## Endpoints (`handler.Register`)

| Method | Path | Đọc/Ghi |
|---|---|---|
| GET | `/healthz` | trả `status` + `write_mode` |
| GET | `/search?q=&category=&status=&brand=&in_stock=&min_price=&max_price=&size=&from=` | **ES** |
| GET | `/products?limit=` | **Postgres** (list cho Admin) |
| POST | `/products` | PG + outbox |
| PUT | `/products/:id` | PG + outbox |
| DELETE | `/products/:id` | PG + outbox |
| POST | `/admin/backfill` | PG → ES `_bulk`, keyset batch 500, idempotent |
| GET | `/admin/reconcile` | COUNT(*) PG vs `_count` ES + outbox_pending (degrade nếu ES down, không 500) |
| GET | `/admin/reconcile/deep?fix=true` | so `updated_at` từng lô id; `fix=true` re-index từ DB |
| POST | `/admin/reindex` | reindex zero-downtime, swap alias |

## Hai chế độ ghi (`WRITE_MODE`)

- **`outbox`** (mặc định): ghi `products` + `outbox` cùng transaction; worker sync ES với **external version** (`updated_at` epoch → 409 = bỏ qua an toàn). Đáng tin, hồi phục được khi ES down.
- **`dual`** (`WRITE_MODE=dual RUN_WORKER=false`): `handler.dualWriteES` ghi thẳng ES trong request; ES down → header `X-Dual-Write-Drift: true`, **KHÔNG rollback PG → lệch vĩnh viễn** (bài học 5.2).

## Chạy

```bash
docker compose up -d elasticsearch postgres     # cần cả 2
cd backend && cp .env.example .env               # optional, default đã khớp
go run .                                          # tự migrate + EnsureIndexAlias + worker
```

Xem thêm overview: skill `es-stack-map`. Verify data: skill `es-data-verify`.
