# CSV / XLSX Streaming với Postgres (file ~1GB)

Ví dụ minh hoạ cách xử lý file **rất lớn** (giả sử ~1GB) mà **không nổ RAM**, cả 2 chiều:

- **Import**: đọc *streaming* file CSV → ghi vào Postgres (batch insert **hoặc** `COPY`).
- **Export**: đọc *streaming* từ Postgres (server-side cursor) → ghi ra file CSV/XLSX.

Kèm 1 script dùng **@faker-js/faker** để tự sinh file giả ~1GB đem test.

---

## Nguyên tắc cốt lõi: STREAMING + BACKPRESSURE

> Không bao giờ giữ cả file/cả bảng trong RAM. Đọc tới đâu, xử lý tới đó, và để
> mắt xích chậm nhất "ghì" tốc độ của cả pipeline lại (backpressure).

| Vấn đề nếu làm sai | Cách làm đúng ở đây |
|---|---|
| `fs.readFileSync` cả file 1GB → nổ heap | `fs.createReadStream` đọc từng chunk |
| Build cả chuỗi 1GB rồi mới ghi | `stream.write()` + chờ `'drain'` (xem `generate-fake-csv.js`) |
| Insert từng dòng (triệu round-trip) | Gom **batch** rồi INSERT nhiều dòng / hoặc `COPY` |
| Đọc file nhanh hơn DB ghi | `for await` tự pause parser → backpressure |
| `SELECT *` nạp hết vào RAM | **pg-query-stream** (server-side cursor) |
| `new Excel.Workbook()` cho data lớn | **`Excel.stream.xlsx.WorkbookWriter`** |

---

## Cấu trúc

```
csv-xlsx-streaming/
├── package.json
├── .env.example            # copy sang .env rồi chỉnh
├── schema.sql              # bảng users
├── db.js                   # pool + init schema (npm run init-db)
├── generate-fake-csv.js    # sinh file CSV giả ~1GB bằng faker (streaming)
├── import-csv-to-db.js     # import: stream parse + batch insert (có transform)
├── import-csv-copy.js      # import: COPY FROM STDIN (nhanh nhất, file sạch)
├── export-db-to-csv.js     # export: cursor → csv-stringify → file
└── export-db-to-xlsx.js    # export: cursor → exceljs streaming writer
```

---

## Chạy thử

### 0. Cài & cấu hình

```bash
cd notes/example/csv-xlsx-streaming
npm install
cp .env.example .env        # sửa DATABASE_URL cho khớp Postgres của bạn
```

Cần 1 Postgres đang chạy. Ví dụ nhanh bằng Docker:

```bash
docker run --name pg-streaming -e POSTGRES_PASSWORD=postgres \
  -e POSTGRES_DB=streaming_demo -p 5432:5432 -d postgres:16
```

### 1. Tạo bảng

```bash
npm run init-db
```

### 2. Sinh file giả (~1GB)

```bash
npm run generate          # đọc FAKE_TARGET_GB trong .env (mặc định 1GB)
```

> 💡 Muốn test nhanh, sửa `FAKE_TARGET_GB=0.05` (~50MB) trước.

### 3. Import file → DB

```bash
npm run import            # batch insert (có chỗ để validate/transform)
# hoặc nhanh hơn nhiều với file sạch:
npm run import:copy       # COPY FROM STDIN
```

### 4. Export DB → file

```bash
npm run export:csv        # ra ./data/export.csv
npm run export:xlsx       # ra ./data/export.xlsx (xem lưu ý XLSX bên dưới)
```

---

## Import: batch insert vs COPY — chọn cái nào?

- **`import-csv-to-db.js` (batch INSERT)** — dùng khi cần **validate / transform / bỏ dòng lỗi**
  trên từng record trước khi ghi. Chỉnh `IMPORT_BATCH_SIZE` trong `.env`
  (mặc định 5000). Batch quá lớn → tốn RAM & dễ vượt giới hạn tham số của 1 lệnh;
  quá nhỏ → nhiều round-trip, chậm.
- **`import-csv-copy.js` (COPY)** — **nhanh nhất**, dùng khi file đã "sạch" và
  đúng thứ tự cột. Đây là cả một pipeline stream thuần `file → COPY → Postgres`,
  không giữ gì trong RAM.

**Mẹo tốc độ cho khối lượng lớn:** cân nhắc `DROP INDEX` (chỉ giữ index cần thiết)
trước khi nạp rồi `CREATE INDEX` lại sau; hoặc nạp vào bảng staging không index.

---

## Lưu ý riêng về XLSX ⚠️

1. **Bắt buộc** dùng `Excel.stream.xlsx.WorkbookWriter` (không dùng `new Excel.Workbook()`)
   cho data lớn — nếu không sẽ dựng cả cây object trong RAM và nổ heap.
2. Mỗi sheet Excel **giới hạn 1.048.576 dòng**. Script tự **tách sheet** khi chạm
   ngưỡng (`MAX_ROWS_PER_SHEET`). Với đúng nghĩa "1GB dữ liệu", **CSV thường là lựa
   chọn đúng** (không giới hạn dòng, nhẹ hơn nhiều). Chỉ chọn XLSX khi số dòng vừa
   phải và cần format/nhiều sheet.

---

## Vì sao RAM luôn "phẳng"?

Tất cả script dùng `stream.pipeline()` hoặc `for await…of` trên stream. Cả hai đều
tự truyền **backpressure** ngược từ mắt xích chậm nhất (đĩa hoặc DB) về nguồn đọc,
nên bộ nhớ chỉ dao động quanh kích thước buffer (vài MB) bất kể file 1GB hay 100GB.

Kiểm chứng nhanh khi đang chạy:

```bash
/usr/bin/time -l node import-csv-copy.js     # macOS: xem "maximum resident set size"
```
