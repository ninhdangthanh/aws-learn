# ES Deep-dive Q&A (Phase 7)

8 câu deep-dive bám mục **§14** của [elasticsearch-middle-notes.md](../elasticsearch-middle-notes.md),
nhưng trả lời gắn với **code đã tự dựng** trong project này (Phase 5–6, đã chạy & test thật).
Mỗi câu: ý cốt lõi → cơ chế → *cách project làm* (file/hành vi verify được).

---

## 1. Đồng bộ DB → ES sao cho đáng tin? Dual-write thiếu gì, outbox giải quyết ra sao? (§14.8)

**Cốt lõi:** DB và ES là hai hệ thống, **không có transaction chung**. Dual-write (ghi DB xong ghi
ES trong cùng request) lệch khi ES fail hoặc process chết giữa chừng: DB có, ES không → search mất doc,
**không tự hồi phục**.

**Outbox** biến "ý định index" thành một dòng trong **cùng transaction DB** với dữ liệu gốc → hoặc cả
hai cùng commit, hoặc cùng rollback. Worker đọc outbox, index sang ES với retry (**at-least-once**).
Để retry an toàn cần **idempotent**: dùng `_id = id của DB` → index lại chỉ ghi đè, không tạo bản trùng.

**Trong project:**
- `store.go` — `CreateProduct/UpdateProduct/DeleteProduct` `INSERT products` + `INSERT outbox` trong **1 tx** (`BEGIN…COMMIT`).
- `outbox/worker.go` — poll `FOR UPDATE SKIP LOCKED` (nhiều worker song song an toàn); ES fail → **không** mark `processed_at` → vòng sau retry.
- Verify: tắt ES → create → `reconcile` báo `outbox_pending>0`, `elasticsearch:"unreachable"`; bật ES → worker tự drain → `in_sync:true`. Dual-write (`WRITE_MODE=dual`) thì tắt ES là **lệch vĩnh viễn** (header `X-Dual-Write-Drift`).

## 2. Chống ghi đè ngược (stale write) khi event tới lệch thứ tự? (§14.8 mở rộng)

**Cốt lõi:** worker chạy song song / retry có thể index **bản cũ sau bản mới** → ES giữ dữ liệu cũ.

**Cơ chế:** external versioning. Gắn version đơn điệu tăng theo thời gian sửa; ES **từ chối (409)**
nếu doc hiện tại đã có version ≥ version gửi lên.

**Trong project:**
- `esclient.IndexDoc` → `PUT /products/_doc/{id}?version={updated_at_epoch_ms}&version_type=external`; `version = Product.UpdatedAt.UnixMilli()`.
- 409 được coi là **OK** (bản mới hơn đã thắng), không phải lỗi → worker mark processed, không kẹt.
- Verify: `scripts/phase5-stale-test.sh` nhét index đảo thứ tự vào outbox → ES trả 409 → giữ bản mới (PASS). Query minh họa: [phase5-sync-alias.http](queries/phase5-sync-alias.http) mục 3–5.

## 3. Reindex đổi mapping mà không downtime? (§14.10)

**Cốt lõi:** không sửa được mapping/analyzer của field đã có → phải tạo index mới rồi chuyển sang.
Nếu app trỏ **thẳng index** thì lúc chuyển sẽ gián đoạn. Giải: app **luôn đọc/ghi qua alias**.

**Cơ chế:** tạo `products_v2` (mapping mới) → `_reindex` từ v1 → **swap alias atomic** trong 1 lệnh
`_aliases` (add v2 + remove v1) → xóa v1. Search không bao giờ thấy khoảng trống vì alias đổi nguyên tử.

**Trong project:**
- `esclient/mapping.go` — `ReindexSwap()` làm đúng 4 bước trên; index mới tên `products_v{unix_ts}`.
- Phase 6 chính là ca dùng thật: đổi mapping (thêm synonym analyzer + `name.suggest` + `tenant_id`) → `POST /admin/reindex` rồi `POST /admin/backfill` nạp lại `_source`. Alias `products` không đổi, FE không biết gì.

## 4. Deep pagination: `search_after` khác `from/size` chỗ nào, vì sao two-phase làm deep paging tốn? (§14.9)

**Cốt lõi:** search là **2 pha** (query phase lấy top-N `_id`+`_score` từ mỗi shard → coordinating gộp;
fetch phase lấy `_source`). Để lấy trang sâu `from=20000`, **mỗi shard** phải trả `from+size=20020` kết
quả cho coordinating sort → RAM/CPU phình theo số shard. ES chặn ở `index.max_result_window` (10000).

**`search_after`** không nhảy offset mà "đi tiếp từ điểm cuối": truyền `sort` values của hit cuối trang
trước làm con trỏ. Điều kiện: **sort ổn định + tie-breaker duy nhất**.

**Trong project:**
- `search.go` — `stableSort = [{_score desc}, {id asc}]`; response trả `next_search_after` = `sort` của hit cuối; FE nút "Tải thêm" gửi lại tham số này.
- ⚠️ tie-breaker dùng field `id` (long), **không dùng `_id`** — ES 8.x cấm fielddata trên `_id`.
- Verify: `q=phone size=2` → page1 ids `[9,13]`, dán `next_search_after` → page2 `[17]`, không trùng.

## 5. Faceted search: vì sao cần `post_filter`? (§14.16)

**Cốt lõi:** e-commerce lọc `brand=Apple` nhưng sidebar vẫn phải hiện count **các brand khác** để user
đổi. Nếu đưa brand vào `query.bool.filter` thì **agg brand chỉ còn Apple** (vì agg tính trên kết quả query).

**Cơ chế:** `post_filter` chạy **sau** aggregation — chỉ cắt **hits** trả về, **không** đụng agg. Nên đặt
brand đang chọn ở `post_filter`, còn agg brand tính trên tập trước post_filter → giữ đủ mọi brand.

**Trong project:**
- `search.go` — `brand` → `post_filter`; các filter khác (tenant, category, status, price) ở `query.bool.filter`; `facetAggs` (terms brand + category) chạy trên query.
- Verify: `brand=Acme` → `hits_brands=[Acme]` nhưng `facet_brand=[Acme, Budget]`.
- Ghi chú: muốn *mỗi* facet giữ count riêng thì cần `filter` aggregation lồng loại đúng filter của chính nó — bản project làm mức đủ minh họa post_filter cho facet đang chọn.

## 6. `track_total_hits` "nói dối" tổng số kết quả thế nào? (§14.15)

**Cốt lõi:** từ ES 7+, mặc định **ngừng đếm chính xác sau 10000** hit để nhanh:
`"total": { "value": 10000, "relation": "gte" }`. UI hiện "10,000 kết quả" mãi không đổi là do bẫy này.

**Cơ chế:** `track_total_hits: true` đếm chính xác (`relation: "eq"`, chậm hơn chút); truyền một số
(vd `50000`) là đếm chính xác tới ngưỡng đó rồi thôi. Thực dụng: để mặc định + hiển thị **"10,000+"**,
chỉ bật `true` khi cần con số tuyệt đối.

**Trong project:**
- `search.go` — `trackTotal()`: `""`/`"true"`→`true`, `"false"`→`false`, số→int; response trả `total_relation`.
- FE render `total_relation === "gte" ? "{n}+" : "{n}"`.

## 7. Highlighting hoạt động ra sao, XSS lưu ý gì? (§14.17)

**Cốt lõi:** ES trả sẵn đoạn khớp bọc `<em>` qua `highlight`. Nguy hiểm: phần text **xung quanh** là
`_source` gốc — có thể chứa HTML/script của người khác nhập → render thẳng là **XSS**.

**Cơ chế chống XSS đúng:** không thể escape *sau* khi ES đã chèn `<em>` (sẽ escape luôn tag của mình).
Dùng **sentinel** làm `pre/post tags`, escape HTML **nguyên đoạn**, rồi mới thay sentinel bằng `<em>` thật
→ text gốc bị vô hiệu hóa, chỉ tag của ta sống sót.

**Trong project:**
- `search.go` — `pre_tags=["\\ue000"] post_tags=["\\ue001"]` (Unicode private-use, không xuất hiện trong text và không bị `html.EscapeString` đụng); `escapeHighlight()` = `html.EscapeString` → `ReplaceAll` sentinel thành `<em>`. FE render bằng `dangerouslySetInnerHTML` an toàn.
- Verify: tạo product tên `<script>alert(1)</script> Evil Laptop` → highlight trả `&lt;script&gt;alert(1)&lt;/script&gt; Evil <em>Laptop</em>` (script bị trung hòa, `<em>` thật).

## 8. Synonyms nên index-time hay search-time? Autocomplete làm sao? Vì sao `search_analyzer` khác index analyzer? (§14.18 + §14.14)

**Synonyms — search-time:** đặt `synonym_graph` ở **`search_analyzer`** để sửa/bổ sung synonym mà
**không phải reindex** toàn bộ document (chỉ cần close/open index nạp lại settings). Đặt ở index-time thì
mỗi lần đổi synonym phải reindex. Ngoài ra `synonym_graph` **chỉ hợp lệ ở search analyzer**.

**Vì sao search_analyzer ≠ index analyzer (autocomplete):** autocomplete kiểu edge-ngram/`search_as_you_type`
sinh nhiều token con lúc **index** ("mac" → "m","ma","mac"…), nhưng lúc **search** phải phân tích query
theo kiểu prefix chứ không ngram lại → nếu dùng cùng analyzer sẽ nổ token và sai match. Nên index và
search dùng analyzer **khác nhau** cho đúng mục đích.

**Trong project:**
- `esclient/mapping.go` — `name`/`description` đặt `search_analyzer: "synonym_search"` (filter `synonym_graph`: laptop↔notebook, phone↔smartphone…); index vẫn analyzer chuẩn.
- `name.suggest` kiểu `search_as_you_type`; `/suggest` (`handler/suggest.go`) query `multi_match type=bool_prefix` trên `name.suggest` + `._2gram` + `._3gram`.
- "Did you mean" = **term suggester** trên `name` (`suggest_mode: always`).
- Verify: `q=notebook` → ra "Acme Laptop Pro" (synonym); `/suggest?q=Globex` → gợi ý tên; `q=labtopp` → `fallback:"suggest"`, `did_you_mean:"laptop"`.

## 9. Admin API vận hành: `backfill` / `reconcile` / `reconcile/deep` / `reindex` dùng khi nào?

**Cốt lõi:** outbox worker chỉ đồng bộ **incremental** (theo thay đổi mới). Khi cần nạp lại **toàn bộ**
dữ liệu, **kiểm tra** độ lệch, hoặc **đổi mapping**, cần bộ API admin riêng — đây là "van xả" cho các tình
huống mà cơ chế đồng bộ thường ngày không tự lo được.

| Endpoint | Khi nào dùng | Cơ chế |
|---|---|---|
| `POST /admin/backfill` | Seed lần đầu, hoặc sau khi ES mất dữ liệu / đổi index mà cần nạp lại `_source` | Quét Postgres theo keyset (batch 500) → `_bulk` sang ES. Idempotent (dùng `_id = id DB`), chạy lại an toàn |
| `GET /admin/reconcile` | Check nhanh "PG và ES có lệch số lượng không" (dashboard/health check định kỳ) | `COUNT(*)` PG vs `_count` ES + số `outbox_pending`. Đọc-only, rẻ. ES down → trả degrade, không 500 |
| `GET /admin/reconcile/deep?fix=true` | Nghi ngờ lệch **nội dung** (không chỉ thiếu/thừa) — vd sau incident, sau bug worker | So `updated_at` từng lô id giữa PG và ES. Tốn hơn reconcile nông. `fix=true` tự re-index doc lệch |
| `POST /admin/reindex` | Đổi mapping/analyzer (field mới, thêm synonym, đổi kiểu dữ liệu…) mà không downtime | Tạo index mới → `_reindex` từ index cũ → swap alias `products` atomic (xem chi tiết §3 ở trên) |

**Trong project:** cả 4 nằm ở `handler/admin.go`, đăng ký trong `handler.go`. Đây chính là cơ chế khắc phục
khi outbox pattern (mục 1) bị trễ/lỗi/lệch — cho phép nạp lại từ đầu, đo độ lệch, và đổi mapping mà không
cần tắt service hay đọc thẳng vào index thật (luôn qua alias).

**Tóm lại outbox vs dual (chi tiết ở mục 1):**
- **`outbox`** (mặc định) — ghi PG + outbox cùng transaction, worker sync ES sau, có external version chống stale write, **tự hồi phục** khi ES down (reconcile sẽ về `in_sync:true` sau khi ES sống lại).
- **`dual`** (`WRITE_MODE=dual`) — ghi thẳng ES trong request, không transaction chung với PG → ES down là **lệch vĩnh viễn** (`X-Dual-Write-Drift`), phải dùng admin API ở trên để vá thủ công. Đây là bài học phản diện, không phải mode dùng thật.

## 10. Outbox có phải cứ "đẩy cho worker" là xong? So với queue kiểu asynq thì sao?

**Cốt lõi:** "đẩy cho worker xử lý" **không phải** điểm cốt lõi của outbox. Cốt lõi là **outbox row được
ghi cùng transaction với data gốc** — cách worker lấy ra xử lý sau đó (poll DB hay đẩy vào queue) chỉ là
chi tiết triển khai phía sau, không quyết định tính đúng đắn.

**So với queue kiểu asynq (Redis-backed):**

| | Outbox (project này) | asynq |
|---|---|---|
| Ghi "ý định sync" | `INSERT products` + `INSERT outbox` **cùng 1 SQL transaction** | `Enqueue()` ghi vào Redis — **transaction riêng**, tách khỏi transaction Postgres |
| Process chết ngay sau khi commit DB | Outbox row đã nằm trong DB, worker poll vẫn thấy → không mất | Nếu chết trước khi `Enqueue()` chạy → job **không tồn tại**, không ai retry → mất vĩnh viễn |

**Vì sao lệch:** Postgres và Redis là hai hệ thống, **không có transaction chung**. `tx.Commit()` xong mới
`Enqueue()` → giữa hai dòng đó chết là mất job. Đây thực chất **cùng lỗi với `WRITE_MODE=dual`** (mục 1, 9)
— chỉ khác đối tượng ghi thẳng không transaction là Redis thay vì ES.

**Cách kết hợp đúng nếu vẫn muốn dùng asynq — "transactional outbox + relay":**
1. Ghi outbox row trong Postgres transaction (như project đang làm).
2. Một **relay process** riêng (poll outbox hoặc CDC) đọc row chưa xử lý → `asynqClient.Enqueue()` → chỉ
   `mark processed_at` **sau khi** enqueue thành công.
3. asynq worker nhận job, xử lý **idempotent** (giống project dùng external version chống stale write — mục 2).

Tóm lại: outbox giải quyết "ghi DB xong, chắc chắn không quên báo hệ thống khác biết". asynq giỏi đoạn
"xử lý job đó thế nào — retry, backoff, nhiều worker song song". Hai cái bổ sung nhau, không thay thế nhau.

**Trong project:** `outbox/worker.go` poll trực tiếp Postgres (`FOR UPDATE SKIP LOCKED`), không dùng
queue ngoài (Redis/asynq) — chọn vậy vì đơn giản, ít moving part, đủ cho mục đích học outbox pattern gốc.

---

## Bonus — Multi-tenant access filter (§14.20)

**Cốt lõi:** search **luôn** phải kèm filter phạm vi truy cập, và filter đó do **backend ép**, không tin
client — quên là lộ dữ liệu chéo tenant.

**Trong project:** `handler.go` `tenantOf()` lấy tenant từ **header `X-Tenant-ID`** (mô phỏng login
context), **không bao giờ** từ body/query. `search.go`/`suggest.go` luôn inject `filter: term tenant_id`;
create *stamp* tenant từ context; update/delete/list *scope* theo tenant.
Verify negative: product của tenant `xss` **không** hiện khi query bằng tenant `acme` dù trùng từ khóa "laptop".
