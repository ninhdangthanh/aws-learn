# Full-Text Search trong PostgreSQL

> Ba file demo chạy được nằm ở `sql/`. Xem `README.md` để biết cách chạy.

## Đặt lại vấn đề cho đúng

Bài toán không phải là *"LIKE chậm, thay bằng full-text search cho nhanh"*.

Bài toán là: **người dùng gõ TỪ, còn `LIKE` so khớp CHUỖI KÝ TỰ.** Hai thứ đó
không phải một, và sai lệch đó gây ra lỗi theo cả hai chiều — vừa thừa vừa thiếu:

```
Người dùng gõ:  "tea"

LIKE '%tea%'  →  Green Tea Sampler        ✓ đúng
                 Black Tea Tin            ✓ đúng
                 Teak Serving Tray        ✗ "teak"
                 Handheld Steam Cleaner   ✗ "steam"
                 Food Steamer Basket      ✗ "steamer"
                 Clip On Thermometer      ✗ "instead"
                 Milk Jug 600ml           ✗ "steaming"

Người dùng gõ:  "machines"

LIKE '%machines%' → Reusable Coffee Filter
                    Water Filter Cartridge
        (trượt hết 9 sản phẩm ghi "machine" số ít, chỉ vì thiếu chữ "s")
```

Tốc độ chỉ là hệ quả đến sau. Kể cả `LIKE` có nhanh vô hạn thì nó vẫn trả sai
kết quả. Đó mới là lý do dùng FTS.

## Ba công cụ, ba bài toán khác nhau

Đây là chỗ dễ nhầm nhất, nên đặt lên trước:

```
                B-tree              FTS (tsvector+GIN)      pg_trgm
              ──────────────       ──────────────────      ──────────────
  đơn vị       giá trị cột           TỪ (đã stem)           3 ký tự liền
  giải          = , < , >             tìm theo từ            chuỗi con
  được          ORDER BY              xếp hạng               gõ sai chính tả
                LIKE 'abc%'           highlight              "ý bạn là...?"

  KHÔNG        LIKE '%abc%'         LIKE '%abc%'            không hiểu
  giải được    tìm theo từ          gõ sai chính tả          nghĩa/ngữ pháp
                                     chuỗi con
```

**FTS không phải bản thay thế của `LIKE '%...%'`.** FTS khớp *từ đầy đủ sau khi
rút gọn về gốc*. Gõ `coff` thì FTS trả về **0 dòng**, dù trong bảng đầy chữ
"coffee". Thứ thay thế được `LIKE '%...%'` là **pg_trgm**, không phải FTS.

Idea ban đầu mở bài bằng `LIKE '%coffee%'` rồi chuyển sang FTS — đó là nhảy sang
một bài toán khác mà không nói. Cần tách rõ hai đường.

## 1. tsvector: Postgres nhìn thấy gì

`to_tsvector()` làm ba việc liên tiếp:

```
"Professional coffee machine for restaurants"
                    ↓  tách từ (tokenize)
   professional | coffee | machine | for | restaurants
                    ↓  bỏ stop word
   professional | coffee | machine |     | restaurants
                    ↓  rút về gốc (stemming)
        'coffe':2 'machin':3 'profession':1 'restaur':5
```

Con số là **vị trí của từ** trong tài liệu — sau này `ts_rank_cd` và
`phraseto_tsquery` dựa vào đó.

Điểm mấu chốt là stemming gộp mọi biến thể về cùng một lexeme:

```
machine  ┐
machines ├──→  'machin'
machined ┘

brewing  ┐
brewed   ├──→  'brew'
brews    ┘
```

Đây chính là thứ `LIKE` không bao giờ có được.

Stemming cũng có thể quá tay: `machined` (gia công) cũng về `machin`. Đổi lại
recall cao hơn nhiều — nói chung là đáng.

## 2. tsquery: câu người dùng gõ biến thành gì

Có 4 hàm và chúng **không** hoán đổi cho nhau được:

| Hàm | `'coffee machine'` cho ra | Dùng khi |
|---|---|---|
| `plainto_tsquery` | `'coffe' & 'machin'` | phải chứa mọi từ, không quan tâm thứ tự |
| `phraseto_tsquery` | `'coffe' <-> 'machin'` | phải đứng liền kề đúng thứ tự |
| `websearch_to_tsquery` | hiểu `"cụm"`, `-loại trừ`, `or` | **input trực tiếp từ người dùng** |
| `to_tsquery` | cú pháp thô `&` `\|` `!` `<->` `:*` | query do code sinh ra |

**`to_tsquery` ném lỗi khi gặp input bẩn.** Đừng bao giờ đưa thẳng chuỗi người
dùng gõ vào nó — một dấu `&` lạc là 500 Internal Server Error.
Mặc định nên dùng `websearch_to_tsquery`.

## 3. Toán tử `@@`

```sql
SELECT * FROM products
WHERE to_tsvector('english', description) @@ plainto_tsquery('english', 'coffee machine');
```

`@@` đọc là "tsvector này có thoả tsquery kia không". Cả hai vế **phải cùng một
text search config** (`'english'`), lệch config là kết quả sai âm thầm.

## 4. Index — và vì sao BẮT BUỘC phải truyền `'english'`

Viết `to_tsvector(description)` một tham số thì index sẽ bị từ chối:

```
ERROR:  functions in index expression must be marked IMMUTABLE
```

Lý do: bản 1 tham số lấy config từ biến `default_text_search_config`, mà biến đó
đổi được lúc runtime → hàm chỉ `STABLE`. Index không được phép chứa hàm có thể
đổi kết quả, vì dữ liệu đã ghi xuống đĩa sẽ thành rác. Truyền `'english'` tường
minh thì hàm mới `IMMUTABLE`.

### Cách cũ: expression index

```sql
CREATE INDEX idx_products_fts ON products
USING GIN (to_tsvector('english', coalesce(name,'') || ' ' || coalesce(description,'')));
```

Chạy được, nhưng có một cái bẫy im lặng: **query phải lặp lại biểu thức y hệt**.
Lệch một `coalesce`, đổi thứ tự cột, thiếu một dấu cách → index không được dùng
và **không ai báo cho bạn biết**. Query chỉ chậm dần đi.

### Cách nên dùng (PostgreSQL 12+): generated column

```sql
ALTER TABLE products
ADD COLUMN search_doc tsvector
GENERATED ALWAYS AS (
    to_tsvector('english', coalesce(name,'') || ' ' || coalesce(description,''))
) STORED;

CREATE INDEX idx_products_search_doc ON products USING GIN (search_doc);
```

Query gọn lại còn:

```sql
SELECT * FROM products
WHERE search_doc @@ websearch_to_tsquery('english', $1);
```

Biểu thức viết **một lần** trong định nghĩa bảng. Postgres tự cập nhật cột khi
`name`/`description` đổi. Đổi lại: tốn thêm dung lượng đĩa.

## 5. Xếp hạng — nửa còn lại của bài toán

Search trả về 200 dòng không sắp xếp thì cũng vô dụng như trả về 0 dòng.

### setweight: title phải nặng hơn body

```sql
setweight(to_tsvector('english', coalesce(title,'')), 'A') ||
setweight(to_tsvector('english', coalesce(body ,'')), 'B')
```

Bốn hạng `A > B > C > D`, mặc định mọi lexeme là `D`. Kết quả trông như
`'replic':2A` — lexeme, vị trí, hạng.

Khác biệt rất thật (số liệu từ `sql/002-ranking.sql`, tìm `"replication"`):

```
                                   không weight    có weight (title=A)
  High Availability Playbook          0.08655           0.34618
  Streaming Replication Setup         0.06079           0.60793   ← đảo ngôi
```

Bài nhắc "replication" 4 lần trong body ban đầu thắng bài có đúng từ đó **trên
tiêu đề**. Có weight thì thứ tự đảo lại — đúng với trực giác người dùng.

Mảng chỉnh trọng số xếp theo thứ tự `{D, C, B, A}` — **ngược bảng chữ cái**, rất
dễ nhầm — và mỗi giá trị phải nằm trong `[0,1]`. Muốn title nặng hơn thì **dìm
D/C/B xuống**, không đẩy A lên (`weight out of range`).

### Chuẩn hoá theo độ dài

Mặc định `ts_rank` không quan tâm tài liệu dài hay ngắn:

```
  id  bài                                      độ dài   norm=0    norm=1
  18  Full Text Search Overview                  110    0.24317   0.05832
  42  The Complete Database Performance Guide    629    0.24317   0.04134
```

Bài dài gấp 6 lần, nhắc "index" đúng một lần, mà điểm y hệt. Tham số thứ 3 là
bitmask cộng dồn được: `1` chia cho `1+log(độ dài)`, `2` chia cho độ dài,
`32` ép về khoảng `(0,1)`. Thực tế hay dùng `1` hoặc `32`.

### ts_rank vs ts_rank_cd

`ts_rank` đếm tần suất. `ts_rank_cd` (cover density) đo cả **khoảng cách** giữa
các từ khoá:

```
  tìm "postgres index"
  "postgres index tuning for busy teams"                      → 0.10000
  "postgres is a ... database ... and it can build an index"  → 0.00625
```

Chênh 16 lần. Cần vị trí từ, nên đừng `strip_tsvector()` nếu định dùng.

### ts_headline: highlight

```sql
ts_headline('english', body, q, 'StartSel=<b>, StopSel=</b>, MaxWords=16')
```

**Rất đắt**: nó phải parse lại toàn bộ văn bản gốc cho *từng* dòng khớp — cột
tsvector đã tính sẵn hoàn toàn không giúp gì, vì highlight cần văn bản gốc chứ
không cần lexeme.

Pattern đúng: lọc → rank → `LIMIT` trong subquery, `ts_headline` chỉ chạy ở
ngoài trên đúng 10-20 dòng của trang hiện tại.

### Cái bẫy lớn nhất: `ts_rank` không index được

```
Limit
  └─ Sort                          ← toàn bộ tập khớp phải chấm điểm rồi sort
       Sort Key: ts_rank(...)
       └─ Bitmap Heap Scan
            └─ Bitmap Index Scan on idx_articles_search_doc
```

GIN index lọc được dòng, nhưng điểm rank phải tính **từng dòng** rồi sort **toàn
bộ**. Nếu `WHERE` khớp 2 triệu dòng thì phải chấm điểm cả 2 triệu rồi mới
`LIMIT 10` được — `LIMIT` không cứu được gì.

Cách xử lý:
- Thêm điều kiện thu hẹp trước (tenant, category, khoảng thời gian).
- Query chặt hơn để tập khớp nhỏ (`AND` thay vì `OR`).
- Cắt bằng CTE: lấy `LIMIT 1000` rồi mới rank trong đó, chấp nhận rank gần đúng.
- Data thật sự lớn → đây là lúc cân nhắc Elasticsearch, không phải lúc tune thêm.

## 6. pg_trgm — đường còn lại

### Trigram là gì

Băm chuỗi thành các mẩu 3 ký tự (viết thường, đệm khoảng trắng hai đầu):

```
"coffee"  →  {"  c", " co", "cof", "off", "ffe", "fee", "ee "}   7 mẩu
"cofee"   →  {"  c", " co", "cof",        "ofe", "fee", "ee "}   6 mẩu
                                                    5 mẩu CHUNG
```

`similarity()` = số mẩu chung / số mẩu hợp. Thuần ký tự, không dính gì tới nghĩa
— và đó chính là lý do nó bắt được lỗi gõ phím.

```
similarity('coffee','coffee')  = 1.000
similarity('coffee','cofee')   = 0.625
similarity('coffee','coffeee') = 0.875
similarity('coffee','tea')     = 0.000
```

### Cứu `LIKE '%...%'` bằng GIN + gin_trgm_ops

```sql
CREATE EXTENSION IF NOT EXISTS pg_trgm;
CREATE INDEX idx_name_trgm ON catalog_items USING GIN (product_name gin_trgm_ops);
```

`gin_trgm_ops` hỗ trợ `LIKE`, `ILIKE`, `~`, `~*` **kể cả khi pattern bắt đầu
bằng `%`**. Đây mới là câu trả lời đúng cho "`LIKE '%coffee%'` chậm".

Giới hạn: **pattern ngắn hơn 3 ký tự thì vô dụng**. `ILIKE '%ug%'` cho ra plan
có `Bitmap Index Scan (rows=50)` + `Rows Removed by Index Recheck: 48` — index
chạy nhưng lọc được 0 dòng. Đặt độ dài tối thiểu 3 ký tự trước khi bắn query.

### Gõ sai chính tả

Đây là chỗ FTS chịu thua hoàn toàn:

```
tìm "cofee grindr"

FTS      →  0 dòng
trigram  →  Conical Burr Coffee Grinder      0.3448
            Flat Burr Coffee Grinder Pro     0.3125
            Hand Coffee Grinder Stainless    0.3030
```

Toán tử `%` = "similarity vượt ngưỡng `pg_trgm.similarity_threshold`" (mặc định
`0.3`). Ngưỡng thấp thì nhiều rác, cao thì bỏ sót — không có con số đúng cho mọi
trường hợp, phải chỉnh theo data thật.

### word_similarity: một từ khoá trong một tên dài

```
similarity     ('grinder', 'Conical Burr Coffee Grinder') = 0.308
word_similarity('grinder', 'Conical Burr Coffee Grinder') = 1.000
```

`similarity()` bị chuỗi dài làm loãng. `word_similarity()` (toán tử `<%`) chỉ so
với đoạn khớp nhất bên trong — đây mới là hàm đúng cho search theo từ khoá lẻ.

### "Ý bạn là...?" — toán tử `<->` + GiST

```sql
SELECT product_name FROM catalog_items
ORDER BY product_name <-> 'expreso machin'
LIMIT 8;
```

Không có `WHERE` — luôn trả về 8 cái gần nhất. Đúng bài toán gợi ý khi search ra
0 kết quả.

**Việc này cần GiST, không phải GIN:**

|  | GIN `gin_trgm_ops` | GiST `gist_trgm_ops` |
|---|---|---|
| `LIKE` / `ILIKE` / `%` | nhanh hơn | chậm hơn |
| `ORDER BY <->` (KNN) | **không hỗ trợ** | hỗ trợ, index trả sẵn thứ tự |
| Kích thước | to hơn | nhỏ hơn |
| Build / update | chậm hơn | nhanh hơn |

Cần cả hai kiểu query thì tạo cả hai index.

## 7. Ghép lại: pattern thật trong production

```sql
WITH fts AS (
    SELECT id, product_name, 'fts' AS matched_by
    FROM catalog_items
    WHERE search_doc @@ websearch_to_tsquery('english', $1)
),
fuzzy AS (
    SELECT id, product_name, 'trigram' AS matched_by
    FROM catalog_items
    WHERE product_name % $1
    ORDER BY similarity(product_name, $1) DESC
    LIMIT 5
)
SELECT * FROM fts
UNION ALL
SELECT * FROM fuzzy WHERE NOT EXISTS (SELECT 1 FROM fts);
```

FTS chạy trước vì nó rẻ và chính xác. Chỉ khi nó ra rỗng mới trả tiền cho
trigram. Người dùng gõ đúng thì không phải chịu chi phí fuzzy.

## Bảng chọn công cụ

| Bài toán | Dùng gì |
|---|---|
| Khớp chính xác / khoảng / `ORDER BY` | B-tree |
| `LIKE 'abc%'` (prefix) | B-tree + `text_pattern_ops`¹ |
| Tìm theo TỪ, hiểu số ít/số nhiều, chia thì | FTS: `tsvector` + GIN |
| Xếp hạng theo độ liên quan | `ts_rank` / `ts_rank_cd` |
| Bôi đậm đoạn khớp | `ts_headline` |
| `LIKE '%chuỗi con%'` phải nhanh | pg_trgm: GIN `gin_trgm_ops` |
| Gõ sai chính tả vẫn ra kết quả | pg_trgm: `%` / `similarity()` |
| "Ý bạn là...?" top-N gần nhất | pg_trgm: `<->` + GiST |
| Autocomplete gõ tới đâu gợi ý tới đó | `to_tsquery('abc:*')` hoặc trigram |

¹ B-tree thường chỉ dùng được cho `LIKE 'abc%'` khi database ở collation `C`.
Với collation khác (`en_US.UTF-8`, `C.UTF-8`...) phải tạo index với
`text_pattern_ops`. Idea ban đầu ghi "B-tree → prefix LIKE" là thiếu vế này.

## Khi nào FTS của Postgres là KHÔNG đủ

Nói thẳng để khỏi mất thời gian sau này. Postgres FTS đủ tốt cho phần lớn ứng
dụng, nhưng nó không có:

- **Fuzzy matching tích hợp trong ranking** — trigram là một đường riêng, phải tự ghép.
- **Xử lý tiếng Việt / CJK** — không có config sẵn cho tiếng Việt. Phải dùng
  `'simple'` + `unaccent`, và mất hoàn toàn stemming.
- **Đồng nghĩa, phân tích cụm từ, learning-to-rank** — có `thesaurus` dictionary
  nhưng phải cấu hình bằng file trên server, không sửa được từ SQL.
- **Rank không sort được bằng index** — mục 5 ở trên. Đây là trần cứng.

Đổi lại, thứ Postgres FTS có mà Elasticsearch không có: **cùng một transaction
với dữ liệu của bạn**. Không có luồng đồng bộ nào để hỏng, không có độ trễ
index. Với hầu hết ứng dụng, đó là đánh đổi đúng.
