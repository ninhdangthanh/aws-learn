# Elasticsearch Middle Backend Notes

File này gom kiến thức Elasticsearch ở mức Middle Backend: đủ để giải thích trong phỏng vấn, biết khi nào nên dùng ES thay vì SQL `LIKE`, hiểu index/mapping/analyzer/query DSL, nắm được kiến trúc phân tán và storage internals, và tránh các lỗi vận hành thường gặp.

Elasticsearch không chỉ là "search cho nhanh". Cần hiểu document được **analyze** thành term, lưu vào **inverted index** bên trong các **segment** bất biến của Lucene, rồi scoring bằng **BM25**, và tất cả được phân tán qua **shard/replica** trên nhiều node. Hiểu được luồng đó thì phần còn lại (mapping, query, relevance, aggregation, vận hành) mới có gốc để bám.

Stack tối thiểu để học và demo: **Elasticsearch + Kibana**. Logstash/Beats chỉ cần khi dựng log pipeline thật (xem [elasticsearch-implement-plan.md](elasticsearchStack/elasticsearch-implement-plan.md)).

Mindset chốt:

> ES là search & analytics engine, **near real-time**, **eventually consistent**, tối ưu cho đọc/tìm/tổng hợp trên dataset lớn. Nó **không** thay database quan hệ khi cần transaction, consistency mạnh và là source of truth. ES đứng **cạnh** DB.

---

## 0. Giải thích nhanh bằng ví dụ đời thường (đọc trước khi vào chi tiết)

Phần này dùng analogy để nắm trực giác trước, các mục 1–15 phía sau mới đi vào thuật ngữ/chi tiết kỹ thuật.

### 0.1 ES là gì — analogy "mục lục cuối sách"

SQL DB giống một **cuốn sách dày**: muốn tìm trang nào chứa từ "con cáo" mà không có mục lục, cách duy nhất là lật từng trang đọc (`LIKE '%cáo%'` = quét toàn bộ, chậm dần theo sách dày lên).

Elasticsearch giống **mục lục kiểu từ điển ở cuối sách**: đọc trước toàn bộ sách, tách ra từng từ, ghi lại "từ X nằm ở trang 5, 12, 88" (đây là **inverted index**, xem §5). Tra từ "cáo" → nhảy thẳng tới đúng trang, không đọc lại cả cuốn. Nhờ tách theo từ, ES còn tiện thể: hiểu "cáo" ~ "con cáo" là cùng ý (đã chuẩn hóa/tách từ), xếp hạng trang nào liên quan nhất (SQL LIKE chỉ trả lời có/không, không xếp hạng), và chịu được gõ sai chính tả.

**Use case gặp thực tế**: ô tìm kiếm sản phẩm (Shopee/Tiki), tìm bài viết, tìm log lỗi hệ thống (ELK), dashboard đếm số liệu real-time, tìm địa điểm gần đây. Danh sách đầy đủ ở §1.

### 0.2 Mapping — analogy "quyết định cách đánh mục lục"

Mapping = schema (giống `CREATE TABLE`), nhưng với field text nó còn quyết định **"từ điển sẽ tách từ ra sao"**. Field `name` = "Áo Thun Nam":

* khai là **`text`** → ES tách thành `["áo", "thun", "nam"]` để search được từng từ, kể cả không dấu;
* khai là **`keyword`** → ES giữ nguyên cả cụm "Áo Thun Nam" làm một khối, dùng để lọc chính xác / sort / group by, **không** tách từ.

Chọn sai loại này là lỗi kinh điển: search không ra kết quả, hoặc sort/group lỗi/tốn RAM. Quy tắc nhanh: mô tả/nội dung dài → `text`; status/email/id/enum → `keyword`. Chi tiết + ví dụ multi-field ở §5 ("`text` vs `keyword`") và §6 (mapping).

### 0.3 Khác gì SQL — ai là "source of truth"

**Source of truth** = nơi giữ dữ liệu thật, đáng tin nhất; mất là mất thật; mọi hệ thống khác phải đồng bộ theo nó.

Analogy: SQL là **sổ cái ngân hàng gốc** — chính xác tuyệt đối, có transaction (chuyển tiền phải all-or-nothing), có ràng buộc. ES là **bản photocopy được đánh index để tra cứu nhanh** — cực nhanh để tìm, nhưng xé mất bản photocopy này thì photocopy lại từ sổ gốc là xong, không mất gì thật.

| | SQL | Elasticsearch |
|---|---|---|
| Transaction ACID | Có | Không (không join thật, không transaction đa document) |
| Ghi xong đọc thấy ngay | Có | Không — trễ ~1s (near real-time, xem §4) |
| Search full-text nhiều field, xếp hạng | Yếu | Mạnh (BM25, xem §8) |
| Mất dữ liệu có tái tạo lại được không | N/A (nó là gốc) | Được — reindex lại từ SQL |

Nguyên tắc chốt: **ES không bao giờ là source of truth**, luôn đứng cạnh SQL. Chi tiết ở §1.

### 0.4 Edge case: dữ liệu ES lệch SQL — và khác Redis chỗ nào

**ES không phải cache.** Redis là cache: giữ **y hệt** dữ liệu SQL để đọc nhanh hơn, có TTL, cache miss thì query lại DB ngay lúc đó (đồng bộ tại chỗ). ES là bản dữ liệu **biến đổi/đánh index** để search, đồng bộ **bất đồng bộ**, trễ vài giây là bình thường — không phải "on-demand" như Redis.

Các tình huống lệch dữ liệu hay gặp (chi tiết cơ chế + code ở §11):

1. **Dual-write inconsistency**: app ghi SQL xong, ghi tiếp ES thì crash giữa chừng → SQL có, ES không → user search không thấy record vừa tạo. Giải bằng **outbox pattern**: ghi dữ liệu + "việc cần đồng bộ" trong cùng 1 transaction SQL, worker riêng đọc outbox rồi đẩy ES, lỗi thì retry → cuối cùng chắc chắn khớp (**eventually consistent**, không phải mất luôn).
2. **Update dồn dập, event tới ES bị đảo thứ tự** → bản cũ ghi đè bản mới. Giải bằng **external version** để ES tự chối bản cũ hơn.
3. **`track_total_hits` "nói dối" tổng số kết quả** (mặc định ES ngừng đếm chính xác sau 10k để nhanh) — không phải bug, là đánh đổi tốc độ lấy độ chính xác (§12.2).

### 0.5 Vận hành: service down có phải ES down theo không?

**Không** — quan hệ phụ thuộc đi **một chiều**:

```text
SQL DB (source of truth) --sync (outbox/CDC)--> Elasticsearch (search index)
        ^                                                |
        |                                                v
  app vẫn chạy bình thường                     chỉ tính năng SEARCH bị ảnh hưởng
  (tạo đơn, thanh toán...)                      nếu ES down
```

* **ES down** → app chính (ghi đơn hàng, thanh toán, CRUD) vẫn chạy vì nó dựa vào SQL, không phải ES. Chỉ tính năng "tìm kiếm" lỗi/tắt tạm — thiết kế tốt là graceful degradation (search báo "đang bảo trì", không kéo sập cả app).
* **App/service ghi dữ liệu chính down** → ES vẫn sống, chỉ là không có dữ liệu mới đổ vào, dữ liệu cũ vẫn search bình thường.
* **Mất hẳn index ES** (vd xóa nhầm) → không phải thảm họa, vì **reindex lại từ SQL** là khôi phục 100% (miễn SQL còn) — đây chính là ý nghĩa "ES không phải source of truth".
* Nguy hiểm thật sự chỉ xảy ra nếu ai đó lỡ coi ES là nơi lưu dữ liệu **duy nhất** (không còn lưu ở SQL) — lúc đó mất ES = mất thật. Đây là lỗi thiết kế cần tránh tuyệt đối (§13, mục "Coi ES là source of truth").

---

## 1. Elasticsearch dùng để làm gì?

Elasticsearch (ES) là search & analytics engine, xây trên **Apache Lucene**, lưu dữ liệu dạng JSON document và truy vấn qua REST API. Lucene lo phần lõi (inverted index, scoring, segment); ES bọc thêm lớp phân tán (shard/replica, cluster, node), REST API, query DSL, aggregation và quản lý vận hành.

Use case phổ biến:

* **full-text search** (sản phẩm, bài viết, người dùng) với ranking theo độ liên quan;
* **autocomplete / search-as-you-type** (edge n-gram, `search_as_you_type`, completion suggester);
* **log & observability** (ELK/Elastic Stack: log của service gom về ES, xem/alert trên Kibana);
* **analytics/aggregation gần real-time** (đếm, group, percentile, histogram theo thời gian, cardinality);
* **geo search** (tìm quanh vị trí, trong polygon — `geo_point`, `geo_shape`);
* **filter phức tạp nhiều field** trên dataset lớn (faceted search / e-commerce filter);
* **vector / semantic search** (ES 8+ hỗ trợ `dense_vector` + kNN cho RAG/semantic).

Khi nào **không** nên dùng ES làm nguồn dữ liệu chính:

* cần **transaction ACID**, quan hệ chặt (join nhiều bảng), tính nhất quán mạnh → PostgreSQL/MySQL vẫn là source of truth. ES không có transaction đa document, không có join thật (chỉ có `nested`/`join` field hạn chế và tốn kém).
* cần đọc-sau-ghi thấy ngay (read-your-write tức thì) → ES near real-time (~1s refresh), không hợp.
* dữ liệu nhỏ, query đơn giản → thêm ES là over-engineering, tốn hạ tầng.

ES thường là **secondary store**: dữ liệu gốc ở SQL/Mongo, đồng bộ sang ES để phục vụ search/analytics.

Câu chốt khi phỏng vấn: *ES không thay DB. ES đứng cạnh DB để giải bài toán full-text search và aggregation mà `LIKE '%...%'` + B-tree index của SQL làm chậm và không rank được. Đánh đổi là mất tính nhất quán mạnh và transaction.*

---

## 2. Vì sao SQL `LIKE '%keyword%'` không đủ?

| Vấn đề của SQL LIKE | ES giải quyết thế nào |
|---|---|
| `LIKE '%x%'` có wildcard đầu → không dùng được B-tree index → **full table scan** | Inverted index: term → danh sách document (posting list), tra O(1)/O(log n) theo term |
| Không hiểu ngôn ngữ (số nhiều, chia động từ, dấu tiếng Việt, hoa/thường) | Analyzer: lowercase, stemming, stop word, bỏ dấu (`asciifolding`), synonyms |
| Chỉ match/không match, **không có điểm liên quan** | BM25 scoring, sort theo relevance mặc định |
| Fuzzy/typo rất khó (`LIKE` không chịu lỗi gõ) | Fuzzy query theo edit distance (Levenshtein), `fuzziness: AUTO` |
| Multi-field search + boost field khó | `multi_match` + boost (`name^3`) |
| Aggregation lớn (group + count distinct + percentile) nặng | Doc values (columnar) + aggregation tối ưu, HyperLogLog cho cardinality |
| Search + filter + facet đồng thời chậm | Filter context có cache, tính bitset nhanh |

Điểm mấu chốt về index: B-tree của SQL sắp theo **giá trị đầy đủ của cột**, nên `LIKE 'abc%'` (prefix) còn dùng được index, nhưng `LIKE '%abc%'` (chứa ở giữa) thì vô hiệu. ES lật ngược lại: nó index theo **từng term đã tách**, nên tìm một từ ở bất kỳ đâu trong text đều nhanh.

Đổi lại, ES **không** mạnh ở: join nhiều bảng, transaction, cập nhật từng field liên tục với độ trễ thấp (mỗi update thực chất là reindex cả document), consistency tức thì.

---

## 3. Khái niệm cốt lõi

```text
Cluster (nhiều node)
  └── Index  (giống "table" logic)
        └── Document (JSON, giống "row")
              └── Field (giống "column")

Index được chia thành Shard (mỗi shard = 1 Lucene index độc lập)
  ├── Primary shard   (ghi vào đây trước)
  └── Replica shard   (bản sao → HA + tăng read throughput)

Mỗi shard (Lucene index) gồm nhiều Segment bất biến
  └── Segment chứa inverted index + doc values + stored fields
```

| Thành phần | Ý nghĩa | Ánh xạ RDBMS (chỉ để dễ hình dung) |
|---|---|---|
| Cluster | Nhóm node cùng tên cluster, chia sẻ dữ liệu | Cụm DB |
| Node | 1 instance ES (JVM process) | 1 server DB |
| Index | Tập document cùng loại | Table |
| Document | 1 bản ghi JSON | Row |
| Field | 1 trường trong document | Column |
| Mapping | Định nghĩa kiểu field + cách analyze | Schema |
| Shard | Phân mảnh vật lý của index (1 Lucene index) | Partition/Shard |
| Replica | Bản sao của primary shard | Read replica |
| Segment | File bất biến bên trong shard | (không có tương đương trực tiếp) |

### Vai trò node (production)

* **Master-eligible**: quản lý cluster state (tạo/xóa index, phân bổ shard). Cần số lẻ (3) để bầu quorum.
* **Data node**: giữ shard, thực thi index/search. Tốn CPU/RAM/disk nhất.
* **Coordinating node**: nhận request, fan-out tới data node, gom kết quả. Mọi node đều có thể làm coordinating.
* **Ingest node**: chạy ingest pipeline (transform document trước khi index).

Cụm nhỏ thì 1 node kiêm hết; cụm lớn tách vai trò để cô lập tải.

### Điểm dễ nhầm

* Số **primary shard** cố định lúc tạo index, **không đổi được** sau đó (phải reindex sang index mới). Có `_split`/`_shrink` nhưng vẫn là tạo index khác.
* Số **replica** đổi runtime được (`PUT /index/_settings {"number_of_replicas": 2}`).
* Mapping của field đã tạo **hầu như không sửa được kiểu**, chỉ thêm field mới → thiết kế mapping cẩn thận từ đầu, hoặc reindex.
* "Type" (`_type`) đã bị bỏ từ ES 7+; giờ mỗi index chỉ 1 loại document.
* Document trong ES **immutable**: "update" thực chất là đánh dấu bản cũ đã xóa (soft delete) + index bản mới. Update nhiều = rác nhiều = merge nhiều.

---

## 4. Storage internals — segment, refresh, flush, merge

Phần này hay bị bỏ qua nhưng là gốc để hiểu vì sao ES near real-time và tại sao update tốn kém.

### Segment bất biến

Một shard (Lucene index) không phải một file duy nhất mà là tập nhiều **segment**. Mỗi segment là một inverted index nhỏ, **bất biến** (immutable) sau khi ghi. Search một shard = search song song tất cả segment rồi gộp kết quả.

Hệ quả của "bất biến":
* Ghi mới → tạo segment mới, không sửa segment cũ → không cần lock đọc, đọc rất nhanh và an toàn concurrency.
* Xóa/Update → không xóa tại chỗ, mà đánh dấu doc là "deleted" trong file `.del`, rồi ghi bản mới ở segment khác. Document rác nằm đó cho tới khi merge.

### Luồng ghi: buffer → refresh → flush

```text
Index request
  -> ghi vào in-memory buffer  +  append vào translog (durability)
  -> refresh (mặc định mỗi 1s):
       buffer -> segment mới (nằm trong OS filesystem cache, chưa chắc trên disk)
       -> segment này đã SEARCH ĐƯỢC   ← đây là lý do "near real-time ~1s"
  -> flush (định kỳ / translog đầy):
       fsync segment xuống disk, xóa translog đã an toàn
```

* **translog (transaction log)**: mỗi thao tác ghi được append vào translog trước. Nếu node chết trước khi flush, ES replay translog lúc khởi động → không mất dữ liệu đã ack. Đây là cơ chế durability.
* **refresh_interval** (mặc định `1s`): quyết định độ trễ "index xong bao lâu thì search thấy". Bulk import nên tăng lên `30s` hoặc `-1` (tắt) rồi refresh 1 lần cuối → nhanh hơn nhiều.
* **flush**: đẩy segment ra disk thật và cắt translog. Người dùng hiếm khi gọi tay.

### Merge

Refresh mỗi giây tạo ra rất nhiều segment nhỏ + segment chứa doc đã xóa. ES chạy **merge** nền: gộp nhiều segment nhỏ thành segment lớn hơn, **thực sự loại bỏ** doc đã đánh dấu xóa.

* Merge tốn I/O và CPU → có thể gây spike latency.
* `_forcemerge` (ép về ít segment) chỉ nên dùng cho index **không còn ghi nữa** (vd log của ngày cũ), tuyệt đối không dùng trên index đang ghi.
* Update/delete nhiều → tỉ lệ doc rác cao → merge liên tục → tốn tài nguyên. Đây là lý do ES không hợp workload update-heavy từng field.

Chốt phỏng vấn: *ES near real-time là do refresh ~1s biến buffer thành segment search được; durability nhờ translog; update tốn kém vì segment bất biến nên phải soft-delete + reindex + merge.*

---

## 5. Inverted index và analyzer — trái tim của ES

### Inverted index

Thay vì lưu "document → nội dung", ES lưu **"term → document nào chứa term đó"** (gọi là **posting list**).

```text
Docs:
  1: "The quick brown fox"
  2: "quick fox jumps"

Inverted index (term -> posting list):
  quick -> [1, 2]
  brown -> [1]
  fox   -> [1, 2]
  jumps -> [2]
```

Mỗi entry posting list còn lưu thêm **term frequency** (số lần term xuất hiện trong doc) và **positions** (vị trí, để `match_phrase` biết thứ tự) → phục vụ scoring và phrase query.

Query "fox" → tra thẳng bảng → trả [1,2]. Không scan document. Đó là lý do search nhanh hơn `LIKE` nhiều bậc.

Ngoài inverted index, mỗi segment còn có:
* **doc values**: lưu dạng **cột** (columnar) cho từng field → phục vụ sort, aggregation, script. Đây là lý do agg/sort nhanh mà không cần load cả document.
* **stored fields** (`_source`): JSON gốc, để trả về cho client.

### Analyzer (analysis pipeline)

Khi index 1 `text` field, ES chạy qua pipeline để ra danh sách **term**:

```text
input text
  -> char filter    (0..n)  vd: bỏ HTML, thay ký tự
  -> tokenizer      (đúng 1) cắt thành token, vd standard/whitespace/ngram
  -> token filter   (0..n)  lowercase, stop word, stemming, asciifolding, synonym...
  -> terms lưu vào inverted index
```

Ví dụ `"The Running Foxes"` với standard analyzer + stemming → `[run, fox]` (đã lowercase, bỏ stop word "the", stem "running"→"run", "foxes"→"fox").

**Quan trọng nhất:** query text cũng được analyze bằng **cùng analyzer** trước khi tra. Đó là lý do search "fox" khớp document chứa "Foxes". Nếu analyzer lúc index và lúc query khác nhau → kết quả sai/không match.

Có thể tách:
* **analyzer** (lúc index) và **search_analyzer** (lúc query) khác nhau khi cần — vd edge n-gram: index thì sinh nhiều prefix, nhưng lúc search **không** n-gram query (nếu không sẽ match bừa).

### Test analyzer nhanh bằng `_analyze`

```json
POST /_analyze
{
  "analyzer": "standard",
  "text": "The Running Foxes"
}
// -> tokens: [the, running, foxes]  (standard KHÔNG stem, chỉ lowercase + tách)
```

Dùng `_analyze` để debug: "vì sao query không match?" — thường do term sinh ra khác kỳ vọng.

### Analyzer dựng sẵn hay gặp

| Analyzer | Hành vi | Dùng khi |
|---|---|---|
| `standard` | Tách theo Unicode, lowercase, bỏ dấu câu | Mặc định, đa số text |
| `simple` | Tách theo chữ cái, lowercase | Text đơn giản |
| `whitespace` | Chỉ tách theo khoảng trắng, **không** lowercase | Giữ nguyên token |
| `keyword` | **Không** tách, cả chuỗi thành 1 term | Field như id, mã |
| `language` (english, ...) | Có stemming + stop word theo ngôn ngữ | Text 1 ngôn ngữ cụ thể |

### Custom analyzer — ví dụ tiếng Việt / e-commerce

```json
PUT /products
{
  "settings": {
    "analysis": {
      "analyzer": {
        "vi_search": {
          "type": "custom",
          "char_filter": [],
          "tokenizer": "standard",
          "filter": ["lowercase", "asciifolding"]   // bỏ dấu: "áo" ~ "ao"
        }
      }
    }
  }
}
```

`asciifolding` giúp "điện thoại" match "dien thoai". Với tiếng Việt nâng cao có thể dùng plugin phân tách từ, nhưng ở mức Middle biết `lowercase` + `asciifolding` là đủ giải thích.

### Autocomplete: edge n-gram

Để gõ "sam" ra "Samsung", index sinh sẵn các prefix:

```text
"Samsung" --edge_ngram(min=2,max=10)--> [sa, sam, samsu, samsung, ...]
query "sam" (KHÔNG n-gram) -> match term "sam"
```

Lưu ý phải đặt `search_analyzer` **không** n-gram, nếu không "sam" bị tách thành [sa, sam] và match cả những thứ chỉ chứa "sa". Với nhu cầu chuẩn có thể dùng field type `search_as_you_type` (ES lo sẵn n-gram).

### `text` vs `keyword` — lỗi kinh điển

| Kiểu | Analyze? | Dùng cho | Ví dụ |
|---|---|---|---|
| `text` | Có (tách term) | Full-text search | mô tả sản phẩm, nội dung bài viết |
| `keyword` | Không (lưu nguyên) | Filter chính xác, sort, aggregation, group | status, tag, email, id, enum, sku |

Một field thường map **cả hai** qua multi-field:

```json
"name": {
  "type": "text",
  "fields": { "raw": { "type": "keyword" } }
}
```

→ `name` để full-text search, `name.raw` để sort/aggregate/filter chính xác.

Các lỗi cụ thể:
* Nhầm `keyword` → dùng `match` mong ranking: `match` trên keyword không tách term, phải khớp nguyên chuỗi.
* Nhầm `text` → dùng `term` mong exact: `term` không analyze, so với term đã lowercase/stem trong index nên gần như không bao giờ khớp đúng chuỗi gốc.
* Sort/aggregate trên `text` thuần → lỗi hoặc phải bật `fielddata` (load term ra heap, **rất tốn RAM**, dễ OOM). Luôn dùng `keyword`/doc values cho sort/agg.

---

## 6. Mapping & index template

### Dynamic mapping

Mặc định ES **tự đoán kiểu** khi gặp field mới (dynamic mapping): string → `text` + `keyword`, số → `long`/`float`, "true/false" → `boolean`, chuỗi giống ngày → `date`.

Rủi ro:
* **Mapping explosion**: document JSON động (vd log có field tùy biến) sinh vô số field → cluster state phình, chậm.
* ES đoán sai kiểu (số điện thoại đoán thành `long`, "12/2020" đoán thành `date`...).

Kiểm soát bằng `dynamic`:
* `dynamic: true` — tự thêm field (mặc định).
* `dynamic: false` — không thêm vào mapping, vẫn lưu trong `_source` nhưng **không search được**.
* `dynamic: strict` — gặp field lạ → **báo lỗi reject document**. An toàn nhất cho schema ổn định.

```json
PUT /products
{
  "mappings": {
    "dynamic": "strict",
    "properties": {
      "name":        { "type": "text", "fields": { "raw": { "type": "keyword" } } },
      "description": { "type": "text" },
      "sku":         { "type": "keyword" },
      "status":      { "type": "keyword" },
      "category":    { "type": "keyword" },
      "price":       { "type": "scaled_float", "scaling_factor": 100 },
      "created_at":  { "type": "date" }
    }
  }
}
```

### Index template

Với index theo thời gian (log-2026-07-01, log-2026-07-02...) không thể tạo mapping tay từng ngày → dùng **index template**: định nghĩa mapping/settings một lần, index nào khớp pattern tên sẽ áp dụng tự động. Kết hợp với **data stream** + **ILM** (Index Lifecycle Management) để tự rollover và xóa index cũ.

---

## 7. Query DSL cơ bản

ES query chia hai nhóm ngữ cảnh (**context**):

* **Query context** — "khớp tới **mức nào**", có tính `_score` (relevance). Trả lời "document này liên quan bao nhiêu".
* **Filter context** — "**có/không**" thỏa điều kiện, **không** tính score, **cache được** (bitset) → nhanh hơn và ổn định. Dùng cho status, khoảng ngày, range, term chính xác, geo.

```json
GET /products/_search
{
  "query": {
    "bool": {
      "must":   [ { "match": { "name": "áo thun nam" } } ],   // query: tính score
      "filter": [
        { "term":  { "status": "active" } },                  // filter: không score, cache
        { "range": { "price": { "gte": 100000, "lte": 500000 } } }
      ]
    }
  },
  "sort": [ "_score", { "created_at": "desc" } ],
  "from": 0,
  "size": 20
}
```

`bool` gồm:

| Mệnh đề | Ý nghĩa | Ảnh hưởng score |
|---|---|---|
| `must` | Phải khớp (AND) | Có |
| `should` | Nên khớp (OR mềm), tăng score | Có |
| `filter` | Phải khớp (AND) | Không, cache |
| `must_not` | Không được khớp (NOT) | Không, cache |

Lưu ý `should`: nếu `bool` **không có** `must`/`filter` thì mặc định cần khớp ít nhất 1 `should` (`minimum_should_match=1`); nếu **đã có** `must`/`filter` thì `should` chỉ để cộng điểm, khớp hay không đều được (trừ khi set `minimum_should_match`).

Các query hay dùng:

* `match` — full-text, **analyze query**, dùng cho `text`. Có `operator: and/or`, `fuzziness`, `minimum_should_match`.
* `term` / `terms` — khớp chính xác, **không analyze**, dùng cho `keyword`/number/date/boolean.
* `match_phrase` — khớp đúng **cụm từ theo thứ tự** (dùng positions), có `slop` cho phép xê dịch.
* `multi_match` — 1 query trên nhiều field: `"fields": ["name^3", "description"]` boost tên gấp 3. Có `type: best_fields / most_fields / cross_fields / phrase`.
* `range` — khoảng số/ngày (`gte`, `lte`, hỗ trợ date math `now-7d/d`).
* `fuzzy` / `match` với `fuzziness: AUTO` — chịu lỗi gõ (edit distance). `AUTO` = tự chọn distance theo độ dài từ.
* `prefix` / `wildcard` / `regexp` — **cẩn thận**, wildcard đầu (`*abc`) rất chậm; autocomplete nên dùng `search_as_you_type` hoặc edge n-gram thay vì `wildcard`.
* `exists` — field có tồn tại/không null.
* `ids` — lấy theo danh sách `_id`.

Ví dụ khác biệt `match` vs `term` (bẫy phỏng vấn):

```json
// status là keyword, giá trị lưu "Active"
{ "term":  { "status": "Active" } }   // KHỚP (so nguyên văn)
{ "term":  { "status": "active" } }   // KHÔNG khớp (khác hoa/thường, term không analyze)
{ "match": { "status": "active" } }   // match sẽ analyze query -> "active", vẫn KHÔNG khớp vì term index là "Active"
```

→ Bài học: với keyword, giá trị lưu sao thì query đúng vậy; muốn không phân biệt hoa thường thì phải chuẩn hóa lúc index (normalizer lowercase) hoặc chọn kiểu phù hợp.

Chốt phỏng vấn: *filter thì cho vào `filter`/`must_not` để tận dụng cache và bỏ scoring; chỉ để trong `must`/`should` những gì thật sự cần rank.*

---

## 8. Relevance / scoring (BM25)

ES 5+ mặc định dùng **BM25** để chấm điểm mỗi document match. Ba tín hiệu:

* **TF** (term frequency): term xuất hiện nhiều trong document → điểm cao, nhưng **bão hòa dần** (khác TF-IDF cũ tăng tuyến tính). Xuất hiện 10 lần không gấp 10 lần xuất hiện 1 lần. Tham số `k1` điều khiển độ bão hòa.
* **IDF** (inverse document frequency): term càng **hiếm** trong toàn index → càng "quý" → điểm cao. Từ phổ biến ("the", "và", "áo") gần như vô giá trị vì xuất hiện ở hầu hết document.
* **Field length norm**: term khớp trong field **ngắn** (title 3 từ) đáng giá hơn field **dài** (body 500 từ). Tham số `b` điều khiển mức phạt độ dài.

Trực giác: "một từ hiếm, xuất hiện trong một field ngắn, vài lần" → điểm cao nhất.

Cách can thiệp relevance:

* **boost field**: `"fields": ["name^3", "description"]` — khớp ở name quý gấp 3.
* **`should`** để cộng điểm cho điều kiện phụ (vd cùng brand, còn hàng).
* **`function_score`**: trộn tín hiệu business vào score — nhân theo độ bán chạy, giảm dần theo thời gian (`gauss` decay cho "mới hơn thì cao hơn"), theo khoảng cách địa lý.
* **`rank_feature` / `rank_features`**: field số dùng làm tín hiệu ranking (pagerank, popularity) hiệu quả hơn function_score.
* **boost lúc query**: `{ "match": { "name": { "query": "...", "boost": 2 }}}`.

Debug điểm số: thêm `"explain": true` vào body search hoặc gọi `GET /index/_explain/{id}` để xem breakdown TF/IDF/norm vì sao document được điểm đó. Rất hữu ích khi "kết quả xếp sai thứ tự".

Lưu ý phân tán: IDF được tính **theo từng shard** (mặc định), nên với index nhiều shard + dữ liệu ít, cùng một query có thể ra điểm hơi khác nhau giữa shard → dùng `search_type=dfs_query_then_fetch` để tính IDF toàn cục khi cần chính xác (đánh đổi thêm 1 round-trip).

---

## 9. Aggregation

ES rất mạnh về analytics gần real-time nhờ **doc values** (columnar). Ba nhóm:

* **Bucket aggregation** — nhóm document (giống `GROUP BY`): `terms`, `date_histogram`, `histogram`, `range`, `filters`, `nested`.
* **Metric aggregation** — tính số trên bucket: `avg`, `sum`, `min`, `max`, `stats`, `cardinality` (distinct count xấp xỉ), `percentiles`, `top_hits`.
* **Pipeline aggregation** — tính trên kết quả agg khác: `derivative`, `cumulative_sum`, `moving_avg`, `bucket_sort`.

Agg **lồng nhau** được (bucket trong bucket, metric trong bucket):

```json
GET /orders/_search
{
  "size": 0,                                  // không cần document, chỉ cần agg
  "aggs": {
    "by_day": {
      "date_histogram": { "field": "created_at", "calendar_interval": "day" },
      "aggs": {
        "revenue":      { "sum": { "field": "amount" } },
        "unique_users": { "cardinality": { "field": "user_id" } }
      }
    }
  }
}
```

Điểm cần nắm:

* `size: 0` = không trả document, chỉ trả kết quả agg → nhanh hơn, đỡ tốn băng thông.
* `cardinality` là **xấp xỉ** (thuật toán **HyperLogLog++**) → nhanh, tốn ít RAM, nhưng **không tuyệt đối chính xác** (sai số ~vài %). Cần nêu rõ khi phỏng vấn: "distinct count trên tỉ bản ghi mà chính xác tuyệt đối thì rất tốn; ES đổi độ chính xác lấy tốc độ/bộ nhớ".
* `terms` agg cũng **xấp xỉ** khi nhiều shard: mỗi shard trả top-N cục bộ rồi gộp → có thể sai count ở phần đuôi. `doc_count_error_upper_bound` cho biết mức sai tối đa. Cần chính xác tuyệt đối thì tăng `shard_size` hoặc composite aggregation.
* `percentiles` (p50/p95/p99) cũng xấp xỉ (thuật toán TDigest) — hợp cho latency dashboard.
* Agg chạy trên tập document **đã match query/filter** → có thể filter trước rồi mới agg (faceted search).

---

## 10. Kiến trúc phân tán — write path, read path, routing

### Routing: document nằm ở shard nào?

Mặc định:

```text
shard = hash(_routing) % number_of_primary_shards
// _routing mặc định = _id
```

Vì công thức có `number_of_primary_shards` ở mẫu số → đổi số primary shard sẽ làm sai routing của toàn bộ document cũ → **đây là lý do không đổi được số primary shard**, phải reindex.

Có thể set `_routing` tùy chỉnh (vd theo `user_id`) để mọi document của một user vào cùng shard → query theo user chỉ chạm 1 shard. Đánh đổi: dễ lệch tải nếu 1 user quá lớn (hot shard).

### Write path (index 1 document)

```text
Client -> Coordinating node
  -> tính routing -> xác định primary shard -> chuyển tới data node giữ primary
  -> Primary: ghi buffer + translog, validate
  -> Primary song song đẩy tới các Replica
  -> Replica ghi xong ack về Primary
  -> Primary ack về Coordinating -> trả client
```

Consistency: mặc định ES chờ primary + đủ replica trong `wait_for_active_shards` ack rồi mới trả về. Đây là ghi **synchronous tới replica** trong shard đó, nhưng **cross-document không có transaction**.

### Read/Search path (2 pha)

```text
Query phase:
  Coordinating -> fan-out query tới 1 bản (primary HOẶC replica) của MỖI shard
  -> mỗi shard tự tìm top-N (chỉ _id + _score), trả về
  -> Coordinating gộp, sort toàn cục -> chốt danh sách _id top-N

Fetch phase:
  Coordinating -> chỉ hỏi các shard giữ đúng những _id đó để lấy _source
  -> gộp -> trả client
```

Hai pha là lý do **deep pagination tốn kém**: để lấy trang thứ 1000 (`from=20000`), **mỗi shard** phải trả `from + size = 20020` kết quả cho coordinating gộp → tốn RAM/CPU cấp số nhân theo số shard.

Replica vừa để **HA** (primary chết → promote replica) vừa để **tăng read throughput** (search chia đều primary/replica). Thêm replica → đọc khỏe hơn nhưng ghi nặng hơn (phải copy nhiều bản) và tốn disk.

### Số shard bao nhiêu là hợp?

* Mỗi shard tốn overhead (bộ nhớ, file handle, thread). Quá nhiều shard nhỏ ("oversharding") → cluster state nặng, phí tài nguyên.
* Shard quá lớn (>50GB) → recovery/rebalance/merge chậm.
* Rule of thumb: nhắm mỗi shard **10–50GB**. Ước lượng tổng data → chia ra số shard, làm tròn có dự phòng tăng trưởng.
* Index theo thời gian (log) → dùng rollover (mỗi shard tới ngưỡng size/age thì tạo index mới) thay vì 1 index khổng lồ.

---

## 11. Đồng bộ dữ liệu từ DB sang ES

ES là secondary store → cần chiến lược sync. Các cách phổ biến:

| Cách | Mô tả | Đánh đổi |
|---|---|---|
| **Dual write** | App ghi DB xong ghi luôn ES | Đơn giản; rủi ro DB ok mà ES fail → **lệch dữ liệu** (dual-write inconsistency) |
| **Outbox + worker** | Ghi event vào bảng outbox **trong cùng transaction DB**, worker đọc và index sang ES với retry | Đáng tin cậy, at-least-once; phức tạp hơn |
| **CDC (Debezium/Kafka)** | Bắt thay đổi từ binlog/WAL, stream sang ES | Tách biệt, scale tốt, không đụng code app; hạ tầng nặng |
| **Reindex định kỳ** | Batch job rebuild toàn bộ index | Đơn giản, chịu độ trễ; hợp dataset nhỏ hoặc bổ sung cho các cách trên |

Vì sao **dual write không đủ**: hai hệ thống (DB, ES) không nằm trong một transaction. Ghi DB xong, process crash trước khi ghi ES → DB có, ES không → search mất document. Không có cách nào làm 2 ghi này atomic nếu chỉ dual write.

**Outbox** giải quyết bằng cách biến "ý định index" thành một dòng trong cùng DB transaction với dữ liệu gốc:

```text
BEGIN;
  INSERT INTO products (...);
  INSERT INTO outbox (aggregate_id, op, payload) VALUES (...);
COMMIT;          -- cả hai cùng thành công hoặc cùng rollback
-- worker đọc outbox -> index ES -> đánh dấu processed; ES fail thì vòng sau retry
```

Điểm production:

* ES là **near real-time**, không real-time: document mới index mặc định ~1s sau mới search được (`refresh_interval`). Sync có thêm độ trễ của worker → tổng độ trễ eventual consistency vài giây là bình thường.
* Index thao tác nên **idempotent**: dùng chính **id của DB làm `_id` document** → index lại (retry) không tạo bản trùng, chỉ ghi đè. Đây là chìa khóa để at-least-once an toàn.
* Xử lý **out-of-order**: nếu event tới lệch thứ tự (update cũ tới sau update mới) có thể ghi đè sai → dùng **external version** (`version_type=external` với `updated_at`/version của DB) để ES từ chối ghi bản cũ hơn.
* **Bulk API** khi sync khối lượng lớn: gộp nhiều thao tác trong 1 request `_bulk` thay vì từng cái → nhanh hơn nhiều lần.
* **Reindex zero-downtime bằng alias**: tạo index mới → `_reindex` → chuyển **alias** trỏ sang index mới (atomic) → xóa index cũ. App **luôn trỏ vào alias**, không trỏ thẳng index (xem chi tiết trong implement-plan).

Liên kết: [event-driven-architecture.md](event-driven-architecture.md) (outbox, CDC, idempotent consumer, at-least-once), [rabbitmq-middle-notes.md](rabbitmq-middle-notes.md) (worker sync, có thể thay poll bằng đẩy qua queue).

---

## 12. Xây dựng search feature thật (feature-level)

Mục 1–11 lo kiến trúc/sync/vận hành. Phần này gom những case **chắc chắn gặp khi làm một API search thật**, thường bị bỏ qua khi chỉ học lý thuyết.

### 12.1 Highlighting — bôi đậm đoạn khớp

Search UI cần hiện đoạn text chứa từ khóa, có đánh dấu. ES trả sẵn qua `highlight`:

```json
GET /products/_search
{
  "query": { "match": { "description": "chống nước" } },
  "highlight": {
    "fields": { "description": {} },
    "pre_tags": ["<em>"], "post_tags": ["</em>"],
    "fragment_size": 120, "number_of_fragments": 1
  }
}
// mỗi hit trả thêm "highlight": { "description": ["... áo <em>chống</em> <em>nước</em> ..."] }
```

Lưu ý: highlight chỉ chạy tốt trên field `text` đã analyze; với text dài nên bật `"type": "unified"` (mặc định) hoặc lưu `term_vector` để nhanh hơn. Nhớ **escape HTML** ở client để tránh XSS khi render.

### 12.2 `track_total_hits` — tổng số kết quả bị "nói dối"

Từ ES 7+, mặc định ES **ngừng đếm chính xác sau 10000** hit (`"total": { "value": 10000, "relation": "gte" }`) để nhanh hơn. Nếu UI hiện "Tìm thấy 10,000 kết quả" mãi không đổi → chính là bẫy này.

```json
{ "track_total_hits": true }        // đếm chính xác (chậm hơn chút)
{ "track_total_hits": 50000 }       // đếm chính xác tới 50k rồi thôi (thỏa hiệp)
```

Thực dụng: trang search hiếm khi cần con số tuyệt đối → để mặc định và hiển thị "10,000+"; chỉ bật `true` khi thật sự cần tổng chính xác.

### 12.3 Faceted search — filter kèm count đúng

E-commerce: lọc "brand = Apple" nhưng sidebar vẫn phải hiện count của **các brand khác** để user đổi lựa chọn. Nếu đưa brand vào `query`/`filter` thì agg brand chỉ còn Apple. Giải bằng `post_filter` + filter aggregation:

```json
GET /products/_search
{
  "query": { "match": { "name": "laptop" } },        // agg tính trên TẬP NÀY
  "aggs": {
    "brands":     { "terms": { "field": "brand" } },
    "categories": { "terms": { "field": "category" } }
  },
  "post_filter": { "term": { "brand": "Apple" } }     // chỉ lọc HITS trả về, KHÔNG ảnh hưởng agg
}
```

Kết quả: hits chỉ còn Apple, nhưng facet `brands` vẫn đủ count mọi brand. Nâng cao: mỗi facet cần loại đúng filter của chính nó → dùng `filter` aggregation lồng cho từng facet.

### 12.4 Synonyms và "did you mean"

**Synonyms** — "iphone" ~ "apple", "laptop" ~ "máy tính xách tay". Dùng `synonym`/`synonym_graph` token filter. Nên đặt ở **search-time** (search_analyzer) thay vì index-time để sửa/bổ sung synonym không phải reindex:

```json
"filter": {
  "my_synonyms": { "type": "synonym_graph", "synonyms": ["iphone, apple phone", "laptop, notebook"] }
}
```

**Gợi ý sửa lỗi / "did you mean"** — dùng suggester:
* `completion suggester` (field type `completion`): autocomplete nhanh, có sẵn trong RAM.
* `term` / `phrase suggester`: gợi ý từ đúng khi user gõ sai ("iphon" → "iphone").

### 12.5 Zero-result fallback + vòng tinh chỉnh relevance

Query ra 0 kết quả là trải nghiệm tệ nhất. Chiến lược nới dần:

```text
match operator=and  -> 0 hit?
  -> hạ xuống operator=or / minimum_should_match "75%"
  -> thêm fuzziness=AUTO (bắt typo)
  -> cuối cùng gợi ý "did you mean" / kết quả phổ biến
```

Tinh chỉnh relevance là **vòng lặp dựa trên data thật**, không phải đoán:
* Log query + kết quả user **click** (hoặc mua) → biết kết quả tốt/xấu.
* Chỉnh boost field, `function_score` (bán chạy/mới/gần), synonyms theo log.
* Kiểm chứng bằng bộ query mẫu + `_explain` trước khi đổi trên production.

### 12.6 Multi-tenancy / access filter — bảo mật ở tầng query

Search luôn phải **kèm filter phạm vi truy cập**; quên là lộ dữ liệu chéo tenant/user. Nguyên tắc: filter này do **backend ép vào**, không tin client:

```json
"query": {
  "bool": {
    "must":   [ { "match": { "name": "$user_query" } } ],
    "filter": [
      { "term": { "tenant_id": "$ctx.tenant_id" } },     // backend gắn, không lấy từ request body
      { "terms": { "visibility": ["public", "$ctx.user_id"] } }
    ]
  }
}
```

Để trong `filter` (không score, cache) là đúng chỗ. Tenant lớn có thể tách index/routing theo tenant; mức Middle biết "luôn ép tenant filter phía server" là đủ.

### 12.7 Giảm payload & tối ưu response

* **`_source` filtering**: chỉ trả field cần cho list (`"_source": ["id","name","price","thumbnail"]`) → nhẹ băng thông, không kéo cả document.
* **`docvalue_fields` / `fields`**: lấy vài field từ doc values thay vì parse `_source`.
* **`stored_fields: false`** khi chỉ cần id để join sang DB lấy chi tiết.

### 12.8 Observability của search (nên biết)

* **Slow log** (`index.search.slowlog`): log query chậm quá ngưỡng để tối ưu.
* Theo dõi: p95/p99 latency search, query rate, tỉ lệ zero-result, cluster health (green/yellow/red), heap/GC, số segment, độ trễ sync DB→ES.
* Alert tối thiểu: cluster `red`, heap cao kéo dài, sync lag tăng, tỉ lệ zero-result tăng đột biến (dấu hiệu relevance/sync hỏng).

---

## 13. Edge case & lỗi production thường gặp

* **Mapping explosion**: index document JSON động, tự sinh quá nhiều field → cluster state phình, master chậm. Đặt mapping tường minh, `dynamic: strict` hoặc `false`, giới hạn `index.mapping.total_fields.limit`.
* **`text` vs `keyword` sai**: filter/sort/aggregate trên `text` sẽ lỗi hoặc cần `fielddata` (load term ra JVM heap → **rất tốn RAM, dễ OOM**). Luôn dùng `keyword`/doc values cho các field đó.
* **Deep pagination**: `from + size` lớn (vd trang 1000) rất tốn vì **mỗi shard** phải gom `from+size` rồi coordinating sort. ES chặn ở `index.max_result_window` (mặc định 10000). Dùng **`search_after`** (kèm sort ổn định, thường sort theo `_id` hoặc timestamp + tie-breaker) cho paginate sâu, hoặc **`scroll`/PIT (Point In Time)** cho export toàn bộ.
* **Refresh quá dày khi bulk**: index tốc độ cao mà refresh mỗi giây → tạo quá nhiều segment nhỏ + merge liên tục → chậm. Bulk import: `refresh_interval: -1` + `number_of_replicas: 0`, xong thì bật lại và force refresh một lần.
* **Shard quá nhiều/quá to**: oversharding → overhead; shard >50GB → recovery/rebalance chậm. Cân số shard theo dung lượng dự kiến (10–50GB/shard).
* **Hot shard**: routing lệch (vd 1 tenant/user chiếm phần lớn dữ liệu) → 1 shard nóng, các shard khác rảnh. Cân nhắc routing key và số shard.
* **Split brain / quorum**: cluster cần đủ master-eligible node (thường 3) và cấu hình quorum (`discovery`, `cluster.initial_master_nodes`) đúng để tránh chia rẽ khi mạng phân mảnh.
* **Circuit breaker / heap**: agg/fielddata/query nặng có thể bung heap → ES có circuit breaker chặn để không OOM, nhưng query bị từ chối. Đặt heap ~50% RAM, tối đa ~30GB (tránh mất compressed oops). Đừng để field values load hết ra heap.
* **Coi ES là source of truth**: mất dữ liệu ES mà không rebuild được từ DB gốc là thảm họa. Luôn có nguồn tái tạo (DB + reindex).
* **Không bảo mật cluster**: ES phơi ra internet không auth (port 9200) từng gây lộ dữ liệu diện rộng nhiều lần. Bật security (auth + TLS), đặt sau firewall/VPC, đừng expose 9200 ra ngoài. ES 8+ mặc định bật security — chỉ tắt ở local học.
* **Cập nhật từng field liên tục**: mỗi update = soft-delete + reindex cả document + merge → workload counter/like tăng liên tục nên để ở DB/Redis, đừng đập thẳng vào ES.

---

## 14. Câu hỏi phỏng vấn hay gặp

1. Khi nào dùng ES thay vì SQL? ES là source of truth được không? Vì sao không?
2. Inverted index là gì, vì sao search nhanh hơn `LIKE '%x%'`? B-tree khác chỗ nào?
3. Analyzer làm gì (char filter → tokenizer → token filter)? Vì sao query cũng phải được analyze bằng cùng analyzer?
4. `text` vs `keyword` khác nhau chỗ nào, khi nào dùng cái nào? Vì sao sort/agg trên `text` tốn RAM?
5. Query context vs filter context? Vì sao filter nhanh hơn (cache, không score)?
6. BM25 tính điểm dựa trên gì (TF bão hòa / IDF / field length norm)? Cách boost relevance?
7. ES near real-time nghĩa là gì? Giải thích refresh, translog, flush, segment, merge.
8. Đồng bộ DB → ES thế nào cho đáng tin? Vì sao dual-write không đủ, outbox giải quyết ra sao?
9. Deep pagination xử lý sao? `search_after` khác `from/size` chỗ nào? Vì sao two-phase search làm deep paging tốn?
10. Reindex không downtime bằng alias làm thế nào?
11. Shard vs replica khác nhau? Đổi được số nào lúc runtime, số nào không, vì sao (routing)?
12. `cardinality` và `terms` agg có chính xác tuyệt đối không? Vì sao xấp xỉ?
13. Vì sao ES không hợp workload update-heavy từng field?
14. Autocomplete làm sao (edge n-gram / search_as_you_type)? Vì sao search_analyzer phải khác index analyzer?
15. `track_total_hits` mặc định làm sai tổng số kết quả thế nào, khi nào cần bật `true`?
16. Faceted search: vì sao cần `post_filter`? Lọc 1 facet mà vẫn giữ count facet khác làm sao?
17. Highlighting hoạt động ra sao? Cần lưu ý gì (field text, XSS)?
18. Synonyms nên đặt index-time hay search-time, vì sao? "Did you mean" dùng gì?
19. Query ra 0 kết quả thì xử lý thế nào? Tinh chỉnh relevance dựa trên gì?
20. Search đa tenant: đảm bảo không lộ dữ liệu chéo bằng cách nào?

---

## 15. Tự đánh giá

Coi như nắm đủ mức Middle khi có thể (tự chấm 2026-07-08 — bằng chứng là project hands-on
[elasticsearchStack](elasticsearchStack/README.md), Phase 1–7 đã chạy thật):

* [x] giải thích vì sao ES tồn tại bên cạnh DB và không thay DB (transaction, consistency, source of truth) — *Postgres = source of truth, ES = secondary store trong project*;
* [x] vẽ luồng **analyzer → inverted index (segment) → BM25**, và luồng **refresh/translog/flush/merge**;
* [x] giải thích near real-time, eventual consistency đến từ đâu — *quan sát trực tiếp: create ở Admin → ~1s sau Search mới thấy (worker + refresh)*;
* [x] viết `bool` query tách đúng `must` (rank) và `filter` (lọc), phân biệt `match` vs `term` — *[phase3-query-dsl.http](elasticsearchStack/queries/phase3-query-dsl.http)*;
* [x] chọn đúng `text`/`keyword` (multi-field) cho một schema thực tế và nói được hệ quả nếu chọn sai — *mapping `products` có `name` text + `name.raw` keyword*;
* [x] giải thích write path / read path (two-phase), routing, và vì sao không đổi được số primary shard;
* [x] nêu chiến lược sync DB→ES, vì sao dual-write lệch, outbox + idempotent (`_id` = id DB) đạt eventual consistency, và reindex bằng alias — *tự tay dựng cả 2 chế độ `dual`/`outbox` + external version, xem [Phase 5](elasticsearchStack/README.md#phase-5--backend-go--gin--sync-db--es)*;
* [x] nói được aggregation nào là xấp xỉ và vì sao — *`cardinality` (HyperLogLog), [phase4-aggregation.http](elasticsearchStack/queries/phase4-aggregation.http)*;
* [x] dựng được một search feature thật: highlighting, `track_total_hits`, faceted search (`post_filter` + facet count), synonyms/suggester, zero-result fallback, multi-tenant access filter, `_source` filtering — *toàn bộ Phase 6, test 7/7 pass, xem [deep-dive-qa.md](elasticsearchStack/deep-dive-qa.md)*;
* [x] kể ít nhất 4 edge case production (mapping explosion, fielddata OOM, deep pagination, oversharding, hot shard, security) và cách phòng — *§13*.

Còn nợ / cần đào sâu thêm (ngoài scope hands-on hiện tại): vận hành cluster thật (multi-node,
shard rebalance, quorum/split-brain), CDC bằng Debezium thay outbox poll, và relevance tuning
dựa trên click-log thực tế.
