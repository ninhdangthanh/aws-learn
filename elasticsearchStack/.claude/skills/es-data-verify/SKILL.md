---
name: es-data-verify
description: >
  Cách kiểm tra dữ liệu Elasticsearch trong project elasticsearchStack có ĐÚNG chưa —
  đối soát Postgres↔ES (count nông, updated_at sâu), soi mapping/alias, check outbox
  pending, external-version 409, và các script verify sẵn có. Dùng khi hỏi "data ES
  đúng chưa", "PG và ES có khớp không", "sao search thiếu/thừa doc", "kiểm tra sync",
  "verify index/mapping". Orientation + chẩn đoán, không sửa code.
---

# Verify dữ liệu Elasticsearch có đúng chưa

Nguyên tắc: **Postgres = source of truth**. "Data ES đúng" = ES khớp Postgres (đã sync hết outbox) và mapping/alias đúng.

## 1. Đối soát nhanh (khuyên dùng đầu tiên)

```bash
curl -s http://localhost:8090/admin/reconcile
# { postgres, elasticsearch, outbox_pending, drift, in_sync }
```
- `in_sync: true` ⇔ `postgres == elasticsearch` **và** `outbox_pending == 0` → OK.
- `outbox_pending > 0` → worker chưa drain xong (đợi vài giây) hoặc worker/ES đang down.
- `elasticsearch: "unreachable"` → ES down (reconcile degrade, không 500).

## 2. Đối soát sâu (phát hiện stale/missing từng doc)

```bash
curl -s "http://localhost:8090/admin/reconcile/deep"          # chỉ báo cáo
curl -s "http://localhost:8090/admin/reconcile/deep?fix=true" # re-index bản lệch từ DB
```
So `updated_at` từng lô id giữa PG và ES → tìm `missing_in_es` / `stale_in_es`. `fix=true` sửa bằng cách re-index từ Postgres.

## 3. Soi trực tiếp ES (Kibana Dev Tools / curl)

```bash
curl -s 'localhost:9200/_cat/aliases/products?v'          # alias products trỏ index nào
curl -s 'localhost:9200/products/_count'                  # số doc (qua alias)
curl -s 'localhost:9200/products/_mapping?pretty'         # mapping thật
curl -s 'localhost:9200/products/_search?size=3&pretty'   # xem vài doc
curl -s -XPOST 'localhost:9200/products/_refresh'         # ép refresh nếu vừa ghi mà chưa thấy
```
Mapping chuẩn (`scripts/products.mapping.json`): `name` text + `name.raw` keyword; `sku/status/category/brand` keyword; `price` scaled_float(100); `in_stock` boolean; `created_at` date.

## 4. Sửa lệch (backfill / reindex)

```bash
curl -s -XPOST http://localhost:8090/admin/backfill   # nạp toàn bộ PG→ES (_bulk, idempotent)
curl -s -XPOST http://localhost:8090/admin/reindex    # reindex zero-downtime, swap alias atomic
```

## 5. Script verify sẵn có

```bash
./scripts/verify.sh              # Phase 1: ES + Kibana sống, cluster health green/yellow
./scripts/phase5-demo.sh         # end-to-end: create→sync→search, drift→recovery, backfill, reindex
./scripts/phase5-stale-test.sh   # external-version chống stale (đảo thứ tự outbox → ES 409 giữ bản mới)
```

## Nguyên nhân lệch thường gặp

- **Chưa refresh**: vừa ghi, ES near-real-time → `_refresh` hoặc chờ ~1s.
- **outbox_pending > 0**: worker chưa chạy (`RUN_WORKER=false`) hoặc ES down → bật lại, worker tự drain.
- **`WRITE_MODE=dual` + ES down giữa chừng**: lệch **vĩnh viễn**, backfill/deep-reconcile mới cứu được (bài học 5.2).
- **409 external version**: bản index cũ hơn bị ES từ chối — **đúng như thiết kế**, không phải lỗi (chống stale write).
- **Tie-breaker sort**: dùng field keyword (`sku`), KHÔNG dùng `_id` (ES 8.x cấm fielddata trên `_id`).

Overview: skill `es-stack-map`. Backend chi tiết: skill `es-backend-map`.
