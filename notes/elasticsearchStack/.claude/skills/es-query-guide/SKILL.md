---
name: es-query-guide
description: >
  Hướng dẫn viết/chạy Query DSL & aggregation trên index products của project
  elasticsearchStack — match vs term, bool must/filter, multi_match boost, match_phrase,
  fuzziness, phân trang from/size & search_after, terms/date_histogram/cardinality/stats,
  đúng theo mapping hiện có. Dùng khi hỏi "viết query", "match vs term", "bool query",
  "aggregation", "phân trang search_after", "sort tie-breaker". Tham chiếu, không thêm feature.
---

# Query DSL & Aggregation — index `products`

Chạy trong **Kibana Dev Tools** (`localhost:5601`, bỏ tiền tố host) hoặc curl `localhost:9200`.
File thật: `queries/phase3-query-dsl.http`, `queries/phase4-aggregation.http`. Seed: `./scripts/seed-products.sh` (30 doc).

## Mapping (quyết định query nào chạy được)

| Field | Type | Dùng cho |
|---|---|---|
| `name` | text (+ `name.raw` keyword) | full-text `match`; sort/agg trên `name.raw` |
| `description` | text | full-text `match` / `match_phrase` |
| `sku`,`status`,`category`,`brand` | keyword | `term`, filter, sort, aggregate (khớp **chính xác**, phân biệt hoa/thường) |
| `price` | scaled_float(100) | `range`, sort, metric agg |
| `in_stock` | boolean | `term` |
| `created_at` | date | `range`, `date_histogram` |

## Query context vs filter context

- **`match`** (text): được analyze → full-text, có `_score`. `"pro laptop"` khớp tài liệu chứa pro/laptop.
- **`term`** (keyword): KHÔNG analyze → khớp nguyên văn. `status:"Active"` (hoa) ra **0 hit** — kinh điển text vs keyword.
- **`bool`**: `must` = query context (tính `_score`); `filter`/`range` = filter context (yes/no, không điểm, **cache được**). Lọc thì luôn để trong `filter`.

```json
GET products/_search
{ "query": { "bool": {
  "must":   [ { "multi_match": { "query": "pro", "fields": ["name^3","description"] } } ],
  "filter": [ { "term": {"status":"active"} }, { "term": {"in_stock":true} },
              { "range": {"price": {"gte":200,"lte":2000}} } ] } },
  "size": 5, "sort": [ {"price":"desc"}, {"sku":"asc"} ] }
```

## Mẹo full-text

- `multi_match` + boost: `"fields": ["name^3","description"]` → khớp ở name quan trọng gấp 3.
- `match_phrase`: cụm liền đúng thứ tự (`"noise cancelling"`).
- `fuzziness: "AUTO"`: chịu typo (`labtop`→laptop). `{ "match": { "name": { "query":"labtop", "fuzziness":"AUTO" } } }`.
- `"explain": true` để đọc vì sao ra `_score` (TF/IDF/field-length norm).

## Phân trang & sort

- `from`/`size`: đơn giản, nhưng deep paging tốn kém.
- `search_after`: phân trang sâu ổn định — dán `sort` values của hit cuối trang trước.
- ⚠️ **Tie-breaker KHÔNG dùng `_id`** (ES 8.x cấm fielddata trên `_id`). Dùng field keyword duy nhất → ở đây `sku`. Sort luôn kèm `{ "sku": "asc" }` làm tie-breaker.

## Aggregation (`"size": 0` = bỏ hits, chỉ lấy agg)

- **Bucket** = nhóm; **Metric** = con số trên nhóm.
- `terms` theo `category`/`brand` (bucket); lồng sub-metric `avg`/`sum` price.
- `date_histogram` `calendar_interval: "month"` trên `created_at` + `sum(price)` → doanh thu theo tháng.
- `cardinality` distinct `brand` (xấp xỉ HyperLogLog).
- `stats` → min/max/avg/sum/count trong 1 lần.
- Kết hợp `query` (filter) + `aggs`: chỉ tính agg trên tập đã lọc.

```json
GET products/_search
{ "size": 0, "aggs": {
  "by_category": { "terms": {"field":"category","size":20},
    "aggs": { "avg_price": {"avg": {"field":"price"}} } } } }
```

Kibana dashboard: `./scripts/kibana-dataview.sh` tạo Data View `products` (timestamp `created_at`) rồi Dashboard → Create (bar theo category, line sum price theo tháng).

Overview: skill `es-stack-map`. Backend `/search` map các param này: skill `es-backend-map`.
