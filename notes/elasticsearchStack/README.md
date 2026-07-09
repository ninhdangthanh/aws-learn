# Elasticsearch Stack — Hands-on

Implement theo [elasticsearch-implement-plan.md](elasticsearch-implement-plan.md).
Lý thuyết: [../elasticsearch-middle-notes.md](../elasticsearch-middle-notes.md).

Stack: **ES 8.14 + Kibana** (Docker) → **Go + Gin** backend (Phase 5-6) → **React/Vite** search UI (Phase 5-6).

## Layout

```
docker-compose.yml   ES (9200) + Kibana (5601). Local học, security tắt.
queries/             File .http / _bulk theo từng phase (chạy bằng curl hoặc Kibana Dev Tools).
scripts/             Script seed / verify.
backend/             (Phase 5) Go + Gin: /search + CRUD (POST/PUT/DELETE), dual-write → outbox worker → alias reindex, deep reconcile.
frontend/            (Phase 5) React + Vite: Search UI (ES) + Admin CRUD (Postgres) + sync status bar.
.claude/skills/      Skills cho Claude Code — orient/tra cứu project (xem "Skills" bên dưới).
```

## Tiến độ (commit theo phase)

- [x] **Phase 1** — Dựng ES + Kibana bằng Docker.
- [x] **Phase 2** — Index, mapping, CRUD document.
- [x] **Phase 3** — Query DSL.
- [x] **Phase 4** — Aggregation + Kibana dashboard.
- [x] **Phase 5** — Backend + sync DB → ES (dual-write → outbox → alias reindex).
- [x] **Phase 6** — Search feature API + React UI (highlight, facet, synonym/suggest, fallback, tenant).
- [x] **Phase 7** — Ghi lại & tổng kết ([deep-dive-qa.md](deep-dive-qa.md) + queries đủ phase + self-assessment §15).

## Phase 1 — Chạy stack

```bash
cd notes/elasticsearchStack
docker compose up -d          # kéo image lần đầu ~1-2 phút

# Đợi tới khi healthy rồi verify:
./scripts/verify.sh
```

Verify thủ công:

```bash
curl -s localhost:9200 | jq              # version + cluster name
curl -s localhost:9200/_cluster/health | jq   # status: green/yellow
```

Kibana: http://localhost:5601 → **Dev Tools** (gõ query trực tiếp).

Dừng stack: `docker compose down` (giữ data) — `docker compose down -v` để xóa cả volume.

## Phase 2 — Index + mapping + seed

```bash
./scripts/seed-products.sh    # xóa+tạo index `products`, bulk 30 docs, verify count
```

- Mapping tường minh: `scripts/products.mapping.json` — `text` cho `name`/`description`,
  `keyword` cho `status`/`category`/`brand`/`sku`, `scaled_float` cho `price`,
  multi-field `name.raw` (keyword) để sort/aggregate.
- CRUD + xem mapping: `queries/phase2-index-mapping-crud.http` (Kibana Dev Tools).
- Vì sao `text` vs `keyword`: `text` được analyze → full-text (`match`);
  `keyword` giữ nguyên → lọc/sort/aggregate chính xác (`term`).

## Phase 3 — Query DSL

File: `queries/phase3-query-dsl.http` (10 query, đã chạy thật trên seed 30 docs).

- `match` (analyzed, có `_score`) vs `term` (keyword, khớp chính xác — `"Active"` ra 0 hit).
- `bool`: `must` (query context, tính điểm) + `filter`/`range` (filter context, không điểm, cache được).
- `multi_match` boost `name^3, description`; `match_phrase` (cụm liền); `fuzziness: AUTO` (typo `labtop`→laptop).
- `explain: true` để đọc TF/IDF/field-length norm.
- Phân trang: `from/size` rồi `search_after` với sort ổn định.
- ⚠️ **Tie-breaker không dùng `_id`** — ES 8.x cấm fielddata trên `_id`
  (`indices.id_field_data.enabled`). Dùng field keyword duy nhất (`sku`) thay thế.

## Phase 4 — Aggregation + Kibana dashboard

File: `queries/phase4-aggregation.http` (đã chạy thật).

- `terms` by category (bucket), lồng sub-metric `avg` price.
- `date_histogram` (monthly) + `sum(price)` → "doanh thu" theo tháng (`size:0`).
- `cardinality` distinct brand (xấp xỉ HyperLogLog).
- `stats` (min/max/avg/sum) trong 1 lần.
- Kibana Data View: `./scripts/kibana-dataview.sh` (tạo `products`, timestamp `created_at`),
  rồi vào **Dashboard → Create** dựng bar chart (category) + line chart (sum price theo tháng).
- Bucket = nhóm (category, khoảng thời gian); Metric = con số trên nhóm (count, sum, avg).

## Phase 5 — Backend (Go + Gin) + sync DB → ES

Postgres = **source of truth**; ES = secondary store cho search. App đọc/ghi ES **qua alias**.

```bash
docker compose up -d postgres elasticsearch kibana   # cần cả Postgres
cd backend && cp .env.example .env                    # optional (default đã khớp)
go run .                                               # :8090, tự migrate + tạo alias + chạy worker

# terminal khác — chạy demo end-to-end:
../scripts/phase5-demo.sh
```

**Endpoints** (`internal/handler`):
- `GET  /search?q=&category=&status=&brand=&in_stock=&min_price=&max_price=&size=&from=` — query ES (không query SQL).
- `POST /products`, `PUT /products/:id`, `DELETE /products/:id` — ghi Postgres (+ outbox cùng transaction).
- `GET  /products?limit=` — list từ Postgres (source of truth) cho Admin UI (phản ánh ngay, không chờ worker).
- `POST /admin/backfill` — nạp toàn bộ Postgres → ES bằng `_bulk`, keyset theo id (idempotent).
- `GET  /admin/reconcile` — đối soát nông: `COUNT(*)` PG vs `_count` ES + outbox pending.
- `GET  /admin/reconcile/deep?fix=true` — đối soát sâu (5.4d): so `updated_at` từng lô id, phát hiện missing/stale, `fix=true` re-index từ DB.
- `POST /admin/reindex` — reindex zero-downtime, swap alias atomic.

**Hai chế độ ghi** (env `WRITE_MODE`):
- `outbox` (mặc định): ghi `products` + `outbox` trong **cùng transaction**; worker poll `FOR UPDATE SKIP LOCKED`,
  index ES với **external version** (`updated_at` epoch → chống stale write, 409 = bỏ qua an toàn). At-least-once + idempotent (`_id` = product id).
- `dual` (`WRITE_MODE=dual RUN_WORKER=false`): ghi PG xong index ES ngay trong request → **tắt ES là lệch vĩnh viễn** (không hồi phục). Đây là bài học 5.2.

**Đã test thật:**
- Happy path: create → worker sync → search thấy; `reconcile in_sync:true`.
- Drift + recovery: tắt ES → `outbox_pending>0`, `elasticsearch:"unreachable"`; bật ES → worker `synced N row(s)` → `in_sync:true`.
- Update phản ánh sang search; delete không để orphan (`reconcile in_sync`).
- External version: PUT bản cũ hơn → ES trả **409** (không đè bản mới).
- Ordering/stale (5.4b): `./scripts/phase5-stale-test.sh` — nhét stale index đảo thứ tự vào outbox → worker bị ES 409 → giữ bản mới (PASS).
- Deep reconcile (5.4d): clobber ES doc → `stale_in_es:1`; `?fix=true` → re-index từ DB, `fixed:1`.
- Backfill idempotent (chạy 2 lần count không đổi); reindex đổi index sau alias mà search không gián đoạn.
- Dual-write: `201 Created` + header `X-Dual-Write-Drift`, ES bật lại vẫn `drift=1` → mất tích vĩnh viễn.

Script: `./scripts/phase5-demo.sh` (outbox + recovery + backfill + reindex), `./scripts/phase5-stale-test.sh` (5.4b).

> Lưu ý: backend tự "chiếm" tên `products` làm alias (xóa index practice Phase 2-4 nếu còn). Muốn chơi lại Phase 2-4 thì chạy `./scripts/seed-products.sh` khi backend chưa chạy.

## Phase 6 — Search feature API thật

Biến query DSL thành API search giống production. Cần **mapping mới** (synonym search-analyzer +
`name.suggest` search_as_you_type + `tenant_id`) nên khi lên từ Phase 5 phải áp lại một lần:

```bash
cd backend && go run .                 # migrate thêm cột tenant_id + tạo alias
curl -X POST localhost:8090/admin/reindex    # index mới với mapping Phase 6
curl -X POST localhost:8090/admin/backfill   # nạp lại _source (điền tenant_id + suggest)

../scripts/phase6-demo.sh              # demo end-to-end (terminal khác)
```

**Endpoint mới / mở rộng:**
- `GET /search` — thêm: `search_after` (JSON array), `track_total_hits`, và luôn có `highlight`,
  `facets{brand,category}`, `fallback`, `did_you_mean`, `next_search_after`, `total_relation`, `tenant`.
- `GET /suggest?q=` — autocomplete (`search_as_you_type` trên `name.suggest`), cũng ép tenant.
- Mọi read đều gửi/nhận header **`X-Tenant-ID`** (default `default`).

**7 kỹ thuật (map với 6.1–6.7):**
- **6.1 Highlight** — `<em>` quanh đoạn khớp. Backend dùng sentinel private-use, `html.EscapeString`
  cả đoạn rồi mới thay bằng `<em>` → text gốc bị vô hiệu hóa (**chống XSS**), FE render an toàn.
- **6.2 Paginate** — `track_total_hits` (`eq` chính xác vs `gte` khi cap ~10000 → UI "10,000+");
  deep paging bằng `search_after` (sort `_score` desc + tie-breaker `id`), không `from` lớn.
- **6.3 Facet** — `terms` agg brand+category; brand chọn bằng **`post_filter`** → hits chỉ brand đó
  nhưng facet brand vẫn đủ mọi brand (agg chạy trước post_filter).
- **6.4 Synonym + suggest** — `synonym_graph` ở **search_analyzer** (sửa synonym chỉ cần close/open,
  không reindex doc): "notebook" → ra "laptop". Autocomplete `bool_prefix`; "did you mean" = term suggester.
- **6.5 Zero-result fallback** — query chính `operator=and`; 0 hit → thử lại `or`+`fuzziness=AUTO`
  (`fallback:"fuzzy"`); vẫn 0 → `did_you_mean` + danh sách phổ biến (`fallback:"suggest"`).
- **6.6 Multi-tenant** — backend **ép** `term tenant_id` từ `X-Tenant-ID` (context), KHÔNG lấy từ body;
  create cũng stamp tenant từ context. Negative: tenant A không bao giờ thấy doc tenant B.
- **6.7 Response** — `_source` chỉ trả field gọn (bỏ `description` — để highlight lo, bỏ `updated_at`);
  query file có bật search slow log.

Query tham chiếu: `queries/phase6-search-feature.http`. Frontend Phase 6: facet click, autocomplete
dropdown, highlight, "Tải thêm" (search_after), "did you mean", chọn tenant.

## Frontend (React + Vite) — Search + Admin CRUD

```bash
cd frontend && cp .env.example .env   # VITE_API_BASE=http://localhost:8090
npm install && npm run dev            # http://localhost:5173  (cần backend chạy)
```

- **Tab Search** → gọi `GET /search` (đọc **Elasticsearch**): box + filter category/brand/status/in_stock/price.
- **Tab Admin CRUD** → `GET /products` (đọc **Postgres**) + form Create/Update/Delete (`POST/PUT/DELETE`).
- **Status bar** poll `/admin/reconcile` mỗi 3s: badge IN SYNC/DRIFT, PG vs ES count, outbox pending, nút Backfill.
- Demo trực quan: tạo product ở Admin (hiện ngay vì đọc SQL) → chuyển tab Search sau ~1s thấy nó (ES qua worker).
  Tắt ES rồi tạo → status bar chuyển **DRIFT** + outbox pending tăng; bật ES → tự về **IN SYNC**.

## Phase 7 — Ghi lại & tổng kết

- **Query theo từng phase** (`queries/`): `phase2-index-mapping-crud`, `phase3-query-dsl`,
  `phase4-aggregation`, `phase5-sync-alias` (alias/external-version/reindex/reconcile — phần ES của luồng sync),
  `phase6-search-feature`. Chạy trong Kibana Dev Tools.
- **Deep-dive Q&A**: [deep-dive-qa.md](deep-dive-qa.md) — 8 câu bám §14 của
  [../elasticsearch-middle-notes.md](../elasticsearch-middle-notes.md), trả lời gắn với code đã build
  (outbox + external version, alias reindex, search_after, post_filter, track_total_hits, highlight-XSS,
  synonym/suggest, tenant filter).
- **Tự đánh giá**: §15 của file notes — tự chấm mức Middle, có bằng chứng trỏ về project này.

## Skills (Claude Code)

`.claude/skills/` chứa các skill để **orient / tra cứu** project khi làm việc với Claude Code
(định hướng, không phải để thêm feature). Gõ `/<tên-skill>` hoặc để Claude tự chọn theo câu hỏi.

| Skill | Dùng khi hỏi |
|---|---|
| `es-stack-map` | "Project này có gì / stack gì / kiến trúc / port / chạy sao" — bản đồ tổng hệ thống. |
| `es-backend-map` | "Backend có gì / endpoint nào / worker chạy sao / config env / package nào làm gì". |
| `es-frontend-map` | "Frontend có gì / component nào / gọi API sao / tab nào làm gì". |
| `es-query-guide` | "Viết query / match vs term / bool / aggregation / phân trang search_after" trên index `products`. |
| `es-data-verify` | "Data ES đúng chưa / PG và ES có khớp không / verify sync / soi mapping, outbox". |
| `es-troubleshoot` | "ES không lên / docker lỗi / port bận / go run fail / reset stack / chạy lại phase". |
