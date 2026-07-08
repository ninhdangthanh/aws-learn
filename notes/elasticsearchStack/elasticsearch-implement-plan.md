# Elasticsearch Stack — Implement Plan

Plan để tự dựng và thực hành Elasticsearch cho phần notes. Mục tiêu: chạy được ES + Kibana bằng Docker, tự tay tạo index/mapping, index dữ liệu thật, viết query DSL và aggregation, rồi nối với một backend nhỏ để hiểu luồng sync DB → ES.

Kiến thức lý thuyết nằm ở [elasticsearch-middle-notes.md](../elasticsearch-middle-notes.md). File này chỉ là plan hành động.

---

## Stack cần dựng — chỉ ES + Kibana là đủ

**Đúng, với mục tiêu học + demo phỏng vấn thì chỉ cần 2 container:**

| Thành phần | Bắt buộc? | Vai trò |
|---|---|---|
| **Elasticsearch** | ✅ Bắt buộc | Search engine + nơi lưu index. Không có thì không có gì để học. |
| **Kibana** | ✅ Rất nên có | UI: Dev Tools console để chạy query, xem index, vẽ dashboard. Học nhanh hơn hẳn so với gõ `curl`. |
| Logstash | ⬜ Optional | Pipeline ingest/transform log. Chỉ cần khi dựng log pipeline thật. |
| Beats (Filebeat...) | ⬜ Optional | Ship log/metric từ máy về ES. Optional như Logstash. |

Kết luận: **ES + Kibana = combo tối thiểu đúng chuẩn.** Logstash/Beats để dành, không cần cho vòng học core này.

> Lưu ý: từ ES 8.x mặc định bật security (TLS + password). Để học cho nhẹ có thể tắt security ở môi trường local (`xpack.security.enabled=false`), nhưng phải hiểu đây chỉ dành cho local, production tuyệt đối không tắt.

---

## Phase 0 — Chuẩn bị (30 phút)

- [x] Cài Docker + Docker Compose.
- [x] Tạo thư mục `notes/elasticsearchStack/` cho code demo (giống cách [multipartS3Upload](../multipartS3Upload/README.md) tổ chức).
- [x] Trên macOS/Linux, tăng `vm.max_map_count` nếu ES báo lỗi bootstrap (`sysctl -w vm.max_map_count=262144`).

---

## Phase 1 — Dựng ES + Kibana bằng Docker (1-2 giờ)

- [x] Viết `docker-compose.yml` với 2 service: `elasticsearch` (port 9200) và `kibana` (port 5601).
- [x] Local học: single node, tắt security cho nhẹ.

```yaml
# notes/elasticsearchStack/docker-compose.yml  (bản local học, KHÔNG dùng production)
services:
  elasticsearch:
    image: docker.elastic.co/elasticsearch/elasticsearch:8.14.0
    environment:
      - discovery.type=single-node
      - xpack.security.enabled=false      # chỉ local
      - ES_JAVA_OPTS=-Xms512m -Xmx512m
    ports:
      - "9200:9200"
    ulimits:
      memlock: { soft: -1, hard: -1 }

  kibana:
    image: docker.elastic.co/kibana/kibana:8.14.0
    environment:
      - ELASTICSEARCH_HOSTS=http://elasticsearch:9200
    ports:
      - "5601:5601"
    depends_on:
      - elasticsearch
```

- [x] `docker compose up -d`.
- [x] Verify ES: `curl localhost:9200` → thấy version + cluster name.
- [x] Verify cluster health: `curl localhost:9200/_cluster/health` → `status: green/yellow`.
- [x] Mở Kibana: http://localhost:5601 → vào **Dev Tools** (console gõ query trực tiếp).

---

## Phase 2 — Index, mapping, CRUD document (2-3 giờ)

Chọn 1 dataset quen thuộc (vd `products`) để bám suốt.

- [x] Tạo index với mapping tường minh: `text` cho `name`/`description`, `keyword` cho `status`/`category`/`sku`, `number` cho `price`, `date` cho `created_at`, multi-field `name.raw` (keyword) để sort.
- [x] Index vài document bằng `PUT /products/_doc/{id}`, dùng **id của DB làm `_id`** (idempotent).
- [x] Bulk index 20-50 document bằng `_bulk` API.
- [x] CRUD: get theo id, update, delete.
- [x] Xem lại mapping tự sinh: `GET /products/_mapping`.

Nghiệm thu: giải thích được vì sao chọn `text` vs `keyword` cho từng field.

---

## Phase 3 — Query DSL (3-4 giờ, phần quan trọng nhất)

- [x] `match` full-text trên `name` → quan sát `_score`.
- [x] `term` trên `status` (keyword) → so sánh với `match` để thấy khác biệt analyze.
- [x] `bool` với `must` (rank) + `filter` (status, `range` giá) → hiểu query vs filter context.
- [x] `multi_match` với boost `name^3, description`.
- [x] `match_phrase`, `fuzziness: AUTO` (typo).
- [x] Thêm `"explain": true` để đọc vì sao document được điểm đó (TF/IDF/field length).
- [x] Phân trang: thử `from/size`, rồi `search_after` với sort ổn định.

Nghiệm thu: viết được 1 query "search + filter + sort + paginate" hoàn chỉnh và giải thích từng mệnh đề.

---

## Phase 4 — Aggregation + Kibana dashboard (2-3 giờ)

- [x] `terms` agg: đếm sản phẩm theo `category`.
- [x] `date_histogram` + `sum`: doanh thu theo ngày (`size: 0`).
- [x] `cardinality`: đếm distinct (hiểu là xấp xỉ HyperLogLog).
- [x] Tạo **Data View** trong Kibana → dựng 1 dashboard đơn giản (bar chart theo category, line chart theo ngày).

Nghiệm thu: đọc được kết quả agg và giải thích bucket vs metric.

---

## Phase 5 — Nối backend + sync DB → ES (bắt buộc, 4-6 giờ)

Đây là phần lõi của việc dùng ES trong thực tế: **PostgreSQL là source of truth**, ES là secondary store phục vụ search. Mục tiêu là tự tay dựng luồng sync đáng tin cậy và hiểu vì sao dual write không đủ.

### 5.1 Backend + source of truth

- [x] Backend nhỏ (**Go + Gin**, chốt) với PostgreSQL là source of truth cho `products`.
- [x] Bảng `products` (id, name, description, price, status, category, created_at, updated_at).
- [x] Endpoint `GET /search?q=...&category=...` → query ES → trả kết quả (KHÔNG query SQL để search).
- [x] Endpoint write CRUD: `POST /products` (create), `PUT /products/{id}` (update), `DELETE /products/{id}` — ghi Postgres + outbox trong cùng transaction.
- [x] Dùng **id của Postgres làm `_id` trong ES** → index lại luôn idempotent, không tạo bản trùng.

### 5.1b Frontend (React + Vite) — chốt scope: Search UI + CRUD admin

FE không có trong plan gốc; bổ sung theo quyết định để demo trực quan.

- [x] Trang **Search**: box tìm kiếm + filter (category/brand/status/in_stock/price) → query ES. (facet count, highlight, autocomplete để **Phase 6**.)
- [x] Trang **Admin CRUD**: form Create / Update / Delete product → gọi `POST/PUT/DELETE` backend.
- [x] Demo consistency nhìn thấy được: tạo/sửa/xóa product ở Admin → Search phản ánh lại; tắt ES → thấy lệch; bật ES → outbox worker tự hồi phục.

### 5.2 Bước 1 — Dual write (làm trước để thấy vấn đề)

Cách đơn giản nhất: app ghi DB xong ghi luôn ES.

```text
POST /products
  -> INSERT INTO products ...        (Postgres, source of truth)
  -> index sang ES (PUT /products/_doc/{id})
  -> trả 201
```

- [x] Cài dual write theo luồng trên.
- [x] **Chủ động tạo lỗi**: tắt ES container rồi tạo product → Postgres đã ghi nhưng ES fail → **dữ liệu lệch** (search không thấy product vừa tạo).
- [x] Ghi nhận: đây là bài toán **dual-write inconsistency** — hai hệ thống, không có transaction chung, một cái thành công một cái fail.

Kết luận rút ra: dual write chỉ hợp demo/dev; production cần cơ chế đảm bảo eventual consistency.

### 5.3 Bước 2 — Outbox + worker (cách đáng tin cậy)

Ý tưởng: ghi thay đổi vào bảng `outbox` **trong cùng transaction** với `products`. Nếu commit thành công thì cả product lẫn "ý định index" đều được lưu atomically. Một worker đọc outbox và index sang ES với retry.

Schema outbox:

```sql
CREATE TABLE outbox (
  id           BIGSERIAL PRIMARY KEY,
  aggregate    TEXT        NOT NULL,   -- 'product'
  aggregate_id TEXT        NOT NULL,   -- product id
  op           TEXT        NOT NULL,   -- 'index' | 'delete'
  payload      JSONB       NOT NULL,   -- snapshot document để index
  created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
  processed_at TIMESTAMPTZ             -- NULL = chưa xử lý
);
CREATE INDEX ON outbox (processed_at) WHERE processed_at IS NULL;
```

Ghi trong cùng transaction:

```text
BEGIN;
  INSERT INTO products (...) VALUES (...);
  INSERT INTO outbox (aggregate, aggregate_id, op, payload)
    VALUES ('product', $id, 'index', $document_json);
COMMIT;   -- cả hai cùng thành công hoặc cùng rollback
```

Worker loop (poll outbox):

```text
loop mỗi ~1s:
  rows = SELECT * FROM outbox
         WHERE processed_at IS NULL
         ORDER BY id
         LIMIT 100
         FOR UPDATE SKIP LOCKED;      -- nhiều worker chạy song song an toàn
  for row in rows:
    if row.op == 'index':  PUT /products/_doc/{row.aggregate_id}  body=row.payload
    if row.op == 'delete': DELETE /products/_doc/{row.aggregate_id}
    UPDATE outbox SET processed_at = now() WHERE id = row.id;
  # ES fail -> KHÔNG update processed_at -> vòng sau retry (at-least-once)
```

- [x] Cài outbox + worker theo trên.
- [x] Lặp lại thí nghiệm tắt ES: giờ product vẫn nằm trong outbox `processed_at IS NULL`, bật ES lên → worker tự index → dữ liệu **tự hồi phục** (eventual consistency).
- [x] Vì index dùng `_id = product id` nên retry index nhiều lần vẫn an toàn (**idempotent**).

Liên kết: [event-driven-architecture.md](../event-driven-architecture.md) (outbox, at-least-once + idempotent consumer), [rabbitmq-middle-notes.md](../rabbitmq-middle-notes.md) (worker pattern, có thể thay poll bằng đẩy qua queue).

> Nâng cao (không bắt buộc): thay poll outbox bằng **CDC** (Debezium đọc WAL của Postgres) đẩy thẳng thay đổi sang ES/Kafka. Chỉ cần biết khái niệm để trả lời phỏng vấn.

### 5.4 Đồng bộ đầy đủ: update, delete, ordering, backfill, đối soát

Outbox ở 5.3 mới lo đường ghi mới. "Hai source luôn đồng bộ" cần thêm:

**Update & delete.** Mọi thay đổi product đều ghi outbox trong cùng transaction, không chỉ create.

```text
UPDATE product -> INSERT outbox(op='index',  payload=snapshot mới)
DELETE product -> INSERT outbox(op='delete', aggregate_id=id)   # worker gọi DELETE /products/_doc/{id}
```

- [x] Test: xóa product ở DB → search trên ES không còn trả nó (không để "document mồ côi").

**Ordering / stale write (chống ghi đè ngược).** Worker chạy song song hoặc retry có thể index bản cũ sau bản mới → ES giữ dữ liệu cũ. Chặn bằng **external version**: lấy `updated_at` (hoặc cột `version` tăng dần) làm version, ES chỉ ghi khi version mới hơn.

```text
PUT /products/_doc/{id}?version={updated_at_epoch}&version_type=external
# ES từ chối (409) nếu document hiện tại đã có version >= version gửi lên -> bản cũ không đè bản mới
```

- [x] Test: bắn 2 update theo thứ tự đảo → ES vẫn giữ bản mới nhất.

**Initial backfill.** Khi mới gắn ES vào DB đã có dữ liệu, phải nạp toàn bộ row hiện có.

- [x] Viết job quét `products` theo trang (keyset theo id) → `_bulk` index sang ES.
- [x] Backfill phải idempotent (dùng `_id` = product id) để chạy lại an toàn.

**Drift detection / reconciliation (mấu chốt).** Dù có outbox, 2 store vẫn lệch được: bug worker, sửa tay DB, ES mất dữ liệu. Vì **SQL là source of truth** nên cần job đối soát định kỳ:

- [x] Job so số lượng: `COUNT(*)` DB vs `_count` ES; cảnh báo nếu lệch.
- [x] Job đối soát sâu hơn: so `updated_at`/checksum theo lô id, cái nào lệch thì re-index từ DB.
- [x] Chấp nhận SQL luôn thắng: khi nghi ngờ, **rebuild ES từ DB** (chính là backfill + alias ở 5.4/5.5) — không bao giờ sửa ngược DB theo ES.

Nghiệm thu mục này: create/update/delete đều phản ánh sang ES; bản cũ không đè bản mới; có backfill và có cách phát hiện + sửa drift.

### 5.5 Reindex zero-downtime bằng alias

Khi cần đổi mapping/analyzer, không sửa được index cũ → phải tạo index mới và reindex. App phải luôn trỏ vào **alias**, không trỏ thẳng index.

```text
# App luôn đọc/ghi qua alias 'products' thay vì index thật
PUT /products_v1
POST /_aliases { "actions": [{ "add": { "index": "products_v1", "alias": "products" }}] }

# Khi đổi mapping:
PUT /products_v2              (mapping mới)
POST /_reindex { "source": { "index": "products_v1" }, "dest": { "index": "products_v2" } }
POST /_aliases {             # chuyển alias atomic trong 1 lệnh
  "actions": [
    { "remove": { "index": "products_v1", "alias": "products" }},
    { "add":    { "index": "products_v2", "alias": "products" }}
  ]
}
DELETE /products_v1          (sau khi verify)
```

- [x] Tạo index qua alias ngay từ 5.1 (app trỏ `products` là alias).
- [x] Thực hành đổi mapping → `_reindex` → chuyển alias → xóa index cũ, đo xem search có gián đoạn không.

Nghiệm thu Phase 5:

* giải thích được dual-write inconsistency bằng thí nghiệm tự tạo;
* vẽ được luồng outbox + worker và vì sao nó đạt eventual consistency + idempotent;
* thực hiện reindex zero-downtime bằng alias không làm gián đoạn search.

---

## Phase 6 — Search feature API thật (bắt buộc, 3-4 giờ)

Phần này biến kiến thức query thành một API search giống production. Tham chiếu lý thuyết: mục 12 trong [elasticsearch-middle-notes.md](../elasticsearch-middle-notes.md).

### 6.1 Highlighting

- [ ] Thêm `highlight` vào query search, trả đoạn khớp có `<em>`.
- [ ] Ở backend: **escape HTML** phần text trước khi bọc tag để tránh XSS khi render.

### 6.2 Phân trang đúng + `track_total_hits`

- [ ] Quan sát mặc định: tổng hit dừng ở `10000` (`relation: gte`) → UI hiện "10,000+".
- [ ] Thử `track_total_hits: true` để lấy tổng chính xác, so sánh latency.
- [ ] Paginate sâu bằng `search_after` (sort ổn định + tie-breaker `_id`), không dùng `from` lớn.

### 6.3 Faceted search (filter + count)

- [ ] Trả về facet `brand`, `category` bằng `terms` agg.
- [ ] Dùng `post_filter` để chọn 1 brand mà **vẫn giữ count các brand khác**.
- [ ] Test: chọn `brand=Apple` → hits chỉ Apple nhưng facet brand vẫn đủ mọi brand.

### 6.4 Synonyms + suggester

- [ ] Cấu hình `synonym_graph` ở **search-time** (search_analyzer) để sửa synonym không phải reindex.
- [ ] Test: search "notebook" ra sản phẩm gắn "laptop".
- [ ] Thêm 1 endpoint autocomplete bằng `completion` suggester hoặc `search_as_you_type`.
- [ ] (tùy chọn) `phrase suggester` cho "did you mean" khi gõ sai.

### 6.5 Zero-result fallback

- [ ] Query chính `operator=and`; nếu 0 hit → hạ `minimum_should_match` / thêm `fuzziness=AUTO`.
- [ ] Nếu vẫn 0 → trả gợi ý "did you mean" hoặc danh sách phổ biến.

### 6.6 Multi-tenant access filter (bảo mật)

- [ ] Backend **ép** `filter: term tenant_id` từ context đăng nhập, KHÔNG lấy từ request body.
- [ ] Test negative: user tenant A không bao giờ thấy document tenant B dù query trùng từ khóa.

### 6.7 Tối ưu response

- [ ] `_source` filtering: list chỉ trả `id, name, price, thumbnail`.
- [ ] (tùy chọn) bật search slow log, xem query nào chậm.

Nghiệm thu Phase 6: có một endpoint `GET /search` trả kết quả có highlight, facet count đúng, hỗ trợ synonym + autocomplete, fallback khi 0 hit, và luôn ép tenant filter.

---

## Phase 7 — Ghi lại & tổng kết (1 giờ)

- [ ] Lưu các query đã chạy vào thư mục `queries/` (đã có `phase2..4-*.http`) hoặc file Kibana export.
- [ ] Cập nhật phần "Tự đánh giá" trong [elasticsearch-middle-notes.md](../elasticsearch-middle-notes.md): tự chấm mục nào đã đạt.
- [ ] Viết 5-8 câu Q&A deep dive cho riêng ES (bám mục 14 của file notes).

---

## Checklist gọn để bắt đầu ngay

1. [ ] `docker compose up -d` (ES + Kibana).
2. [ ] `curl localhost:9200/_cluster/health`.
3. [ ] Kibana Dev Tools tạo index `products` + mapping.
4. [ ] `_bulk` index dataset.
5. [ ] Viết `bool` query đầu tiên (match + filter + range).
6. [ ] 1 aggregation + 1 dashboard.
7. [ ] Nối backend + sync DB→ES: dual write → outbox + worker → reindex bằng alias (Phase 5, phần lõi).
8. [ ] Search API thật: highlight + facet (`post_filter`) + synonym/suggest + zero-result fallback + tenant filter (Phase 6).
