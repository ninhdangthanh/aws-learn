# Elasticsearch Stack — Hands-on

Implement theo [../elasticsearch-implement-plan.md](../elasticsearch-implement-plan.md).
Lý thuyết: [../elasticsearch-middle-notes.md](../elasticsearch-middle-notes.md).

Stack: **ES 8.14 + Kibana** (Docker) → **Go + Gin** backend (Phase 5-6) → **React/Vite** search UI (Phase 5-6).

## Layout

```
docker-compose.yml   ES (9200) + Kibana (5601). Local học, security tắt.
queries/             File .http / _bulk theo từng phase (chạy bằng curl hoặc Kibana Dev Tools).
scripts/             Script seed / verify.
backend/             (Phase 5+) Go + Gin: /search + CRUD (POST/PUT/DELETE), dual-write → outbox worker → alias reindex.
frontend/            (Phase 5+) React + Vite: Search UI (facet, highlight, autocomplete) + Admin CRUD (create/update/delete).
```

## Tiến độ (commit theo phase)

- [x] **Phase 1** — Dựng ES + Kibana bằng Docker.
- [x] **Phase 2** — Index, mapping, CRUD document.
- [x] **Phase 3** — Query DSL.
- [ ] Phase 4 — Aggregation + Kibana dashboard.
- [ ] Phase 5 — Backend + sync DB → ES (dual-write → outbox → alias reindex).
- [ ] Phase 6 — Search feature API + React UI.
- [ ] Phase 7 — Ghi lại & tổng kết.

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
