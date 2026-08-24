# PostgreSQL Full-Text Search — demo chạy được

Ba file SQL độc lập, mỗi file tự tạo bảng, nạp 50 dòng, rồi chạy các query demo
có chú thích. Đọc `idea.md` trước để nắm lý thuyết, chạy file SQL để thấy nó thật.

## Chạy

Cần một Postgres 12+ bất kỳ (file 003 cần thêm quyền tạo extension `pg_trgm`).

```bash
# Postgres có sẵn trên máy
psql -d mydb -f sql/001-core-fts.sql
psql -d mydb -f sql/002-ranking.sql
psql -d mydb -f sql/003-trigram.sql
```

Không có sẵn thì dựng một cái tạm:

```bash
docker run -d --name fts_lab \
  -e POSTGRES_USER=app -e POSTGRES_PASSWORD=app -e POSTGRES_DB=fts_lab \
  -p 55432:5432 postgres:16-alpine

# đợi vài giây cho nó lên
for f in sql/*.sql; do docker exec -i fts_lab psql -U app -d fts_lab -q < "$f"; done

# dọn
docker rm -f fts_lab
```

Ba file dùng ba bảng khác nhau (`products`, `articles`, `catalog_items`) nên chạy
chung một database thoải mái, thứ tự nào cũng được.

## Đọc kết quả thế nào

Mỗi file in ra xen kẽ:

- `=== x.y  Tiêu đề ===` — bắt đầu một demo
- bảng kết quả query
- `-->` — giải thích **kết quả vừa in ra** nghĩa là gì

Chỗ đáng dừng lại là các dòng `-->`. Bảng kết quả chỉ là bằng chứng.

## Ba file demo gì

### `001-core-fts.sql` — 50 sản phẩm cà phê

Vì sao cần FTS, và FTS *không* làm được gì.

| Mục | Nội dung |
|---|---|
| 1 | `LIKE` sai ở đâu: tìm `tea` ra `steam`/`instead`/`teak`; tìm `machines` trượt `machine`; phụ thuộc thứ tự từ |
| 2 | `to_tsvector` biến câu thành lexeme + vị trí; 4 hàm `*_tsquery` khác nhau ra sao |
| 3 | FTS sửa cả 3 lỗi ở mục 1 |
| 4 | Vì sao `to_tsvector(description)` 1 tham số **không index được** (chạy thật, in ra lỗi); expression index vs generated column |
| 5 | `EXPLAIN` chứng minh GIN index được dùng |
| 6 | Giới hạn: gõ `coff` → 0 dòng; gõ sai `cofee` → 0 dòng. Dẫn sang 003 |

### `002-ranking.sql` — 50 bài viết kỹ thuật

001 trả lời *"dòng nào khớp"*. File này trả lời *"dòng nào khớp nhất"*.

| Mục | Nội dung |
|---|---|
| 1 | Kết quả không xếp hạng thì vô dụng |
| 2 | `setweight` A/B cho title/body — thứ tự kết quả **đảo ngược** khi có weight |
| 3 | Chuẩn hoá theo độ dài tài liệu; `ts_rank` vs `ts_rank_cd` (chênh 16 lần) |
| 4 | `ts_headline` highlight, và vì sao nó đắt |
| 5 | **`ts_rank` không index được** — cái trần cứng của Postgres FTS |

### `003-trigram.sql` — 50 mặt hàng

Chỗ idea gốc hiểu nhầm. FTS **không** cứu được `LIKE '%...%'` và **không** cứu
được lỗi chính tả — pg_trgm mới làm việc đó.

| Mục | Nội dung |
|---|---|
| 1 | Trigram là gì; `show_trgm()`, `similarity()` |
| 2 | GIN + `gin_trgm_ops` biến `ILIKE '%x%'` từ Seq Scan thành Bitmap Index Scan; giới hạn pattern < 3 ký tự |
| 3 | Gõ sai `cofee grindr`: FTS 0 dòng, trigram ra 3 dòng đúng. `word_similarity` / `<%` |
| 4 | `<->` + GiST cho "ý bạn là...?"; bảng so sánh GIN vs GiST |
| 5 | Ghép FTS + trigram: pattern fallback dùng thật trong production |

## Lưu ý về data 50 dòng

**Các demo này không đo tốc độ, và cố tình không đo.**

Với 50 dòng, Postgres luôn chọn Seq Scan — vì đó là lựa chọn *đúng*: đọc hết một
bảng nằm gọn trong 1-2 page rẻ hơn đi qua index. Một benchmark trên 50 dòng chỉ
đo được nhiễu.

Nên các demo chứng minh hai thứ khác, và đó là hai thứ thật sự quan trọng khi
đang dev:

1. **Hành vi** — `LIKE` và FTS trả về *tập kết quả khác nhau*, khác ở đúng dòng nào.
2. **Plan** — index *có dùng được không*, kiểm tra bằng `SET enable_seqscan = off`
   rồi đọc `EXPLAIN`. Thấy `Bitmap Index Scan on <tên index>` nghĩa là trên bảng
   vài triệu dòng, planner sẽ tự chọn nó.

Đây cũng là cách đúng để verify index khi bạn đang dev trên data nhỏ: đừng tin
`EXPLAIN` mặc định nói "Seq Scan" là index hỏng.
