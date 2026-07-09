---
name: es-frontend-map
description: >
  Bản đồ frontend React + Vite của project elasticsearchStack — cấu trúc component,
  lớp gọi API (api.ts), 2 tab Search(ES)/Admin(Postgres) + StatusBar reconcile, env,
  cách chạy. Dùng khi hỏi về FE: "frontend có gì", "component nào", "gọi API sao",
  "tab nào làm gì", "chạy frontend thế nào". Orientation, không phải thêm feature.
---

# Frontend (React + Vite) — Bản đồ

`es-stack-frontend`, React 18 + Vite 5 + TypeScript, cổng **:5173**. Gọi backend Go qua `VITE_API_BASE` (mặc định `http://localhost:8090`). **Không có state lib** — chỉ `useState`.

## Cấu trúc (`frontend/src/`)

| File | Vai trò |
|---|---|
| `main.tsx` | mount `<App/>` |
| `App.tsx` | 2 tab: **Search (ES)** / **Admin CRUD (Postgres)** + `<StatusBar/>`. `refreshKey` bump sau mỗi CRUD → StatusBar reconcile lại |
| `SearchPage.tsx` | box + filter (category/brand/status/in_stock/price) → `api.search` → đọc **Elasticsearch** |
| `AdminPage.tsx` | list + form Create/Update/Delete → `api.list/create/update/remove` → **Postgres**; `onChanged` báo App bump |
| `StatusBar.tsx` | poll `api.reconcile` mỗi 3s: badge IN SYNC/DRIFT, PG vs ES count, outbox pending, nút Backfill |
| `api.ts` | lớp fetch tới backend + types `Product`, `SearchResult`, `Reconcile`, `SearchParams` |
| `styles.css` | style |

## Lớp API (`api.ts`)

`BASE = import.meta.env.VITE_API_BASE || "http://localhost:8090"`. Helper `req<T>` set JSON header, throw khi `!res.ok`, trả `undefined` cho 204.

| Hàm | Gọi | Nguồn |
|---|---|---|
| `search(params)` | `GET /search?...` | ES |
| `list(limit=50)` | `GET /products?limit=` | Postgres |
| `create/update/remove` | `POST/PUT/DELETE /products` | Postgres (+outbox) |
| `reconcile()` | `GET /admin/reconcile` | đối soát PG↔ES |
| `backfill()` | `POST /admin/backfill` | nạp PG→ES |

## Ý nghĩa demo trực quan

Tạo product ở **Admin** → hiện ngay (đọc SQL). Chuyển **Search** sau ~1s → thấy nó (ES qua worker).
Tắt ES rồi tạo → StatusBar chuyển **DRIFT** + outbox pending tăng; bật ES → tự về **IN SYNC**.

## Chạy

```bash
cd frontend && cp .env.example .env    # VITE_API_BASE=http://localhost:8090
npm install && npm run dev             # http://localhost:5173  (cần backend :8090 chạy)
```

Backend map: skill `es-backend-map`. Overview: skill `es-stack-map`.
