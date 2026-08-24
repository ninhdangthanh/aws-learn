-- =====================================================================
-- 003-trigram.sql — pg_trgm: thứ THẬT SỰ thay thế được LIKE '%...%'
--
--   Chạy:  psql -d <db> -f sql/003-trigram.sql
--   Cần:   CREATE EXTENSION pg_trgm  (quyền superuser hoặc đã whitelist)
--
--   Đây là chỗ idea gốc hiểu nhầm. Full-text search KHÔNG cứu được
--   LIKE '%coffee%' và KHÔNG cứu được lỗi chính tả. FTS khớp TỪ đã stem.
--   Bài toán chuỗi con và bài toán gần đúng thuộc về trigram.
-- =====================================================================

\set ON_ERROR_STOP on
\pset pager off

CREATE EXTENSION IF NOT EXISTS pg_trgm;

DROP TABLE IF EXISTS catalog_items;

CREATE TABLE catalog_items (
    id           BIGSERIAL PRIMARY KEY,
    sku          TEXT NOT NULL,
    product_name TEXT NOT NULL
);

INSERT INTO catalog_items (sku, product_name) VALUES
('ESP-1001', 'Espresso Coffee Machine Professional'),
('ESP-1002', 'Espresso Coffee Machine Compact'),
('ESP-1003', 'Espresso Machine Portafilter 58mm'),
('GRD-2001', 'Conical Burr Coffee Grinder'),
('GRD-2002', 'Flat Burr Coffee Grinder Pro'),
('GRD-2003', 'Hand Coffee Grinder Stainless'),
('BRW-3001', 'Cold Brew Coffee Maker 1.5L'),
('BRW-3002', 'Pour Over Coffee Dripper Glass'),
('BRW-3003', 'French Press Coffee Maker 800ml'),
('BRW-3004', 'Siphon Coffee Brewer Halogen'),
('BRW-3005', 'Moka Pot Stovetop Coffee Maker'),
('MLK-4001', 'Electric Milk Frother Handheld'),
('MLK-4002', 'Electric Milk Frother Pro'),
('MLK-4003', 'Automatic Milk Steamer 500ml'),
('MLK-4004', 'Stainless Milk Pitcher 600ml'),
('MLK-4005', 'Barista Milk Jug 350ml'),
('TEA-5001', 'Cast Iron Tea Pot 900ml'),
('TEA-5002', 'Glass Tea Infuser Mug'),
('TEA-5003', 'Bamboo Matcha Whisk Set'),
('TEA-5004', 'Electric Tea Kettle Variable Temp'),
('ACC-6001', 'Digital Coffee Scale 0.1g'),
('ACC-6002', 'Espresso Tamper Flat Base'),
('ACC-6003', 'Knock Box Espresso Drawer'),
('ACC-6004', 'Coffee Bean Storage Canister'),
('ACC-6005', 'Reusable Coffee Filter Mesh'),
('ACC-6006', 'Paper Coffee Filters 100pk'),
('ACC-6007', 'Descaling Solution Coffee Machines'),
('ACC-6008', 'Group Head Cleaning Brush'),
('ACC-6009', 'Backflush Cleaning Tablets'),
('ACC-6010', 'Water Filter Cartridge Twin Pack'),
('CUP-7001', 'Ceramic Cappuccino Cup 220ml'),
('CUP-7002', 'Double Wall Espresso Glass 80ml'),
('CUP-7003', 'Vacuum Travel Tumbler 350ml'),
('CUP-7004', 'Insulated Coffee Flask 1L'),
('CUP-7005', 'Porcelain Cupping Bowl Set'),
('BEA-8001', 'Ethiopia Yirgacheffe Beans 1kg'),
('BEA-8002', 'Colombia Huila Beans 500g'),
('BEA-8003', 'Brazil Santos Beans 1kg'),
('BEA-8004', 'Decaffeinated Beans Swiss Water'),
('BEA-8005', 'House Blend Coffee Beans 1kg'),
('BEA-8006', 'Single Origin Coffee Beans Kenya'),
('SYR-9001', 'Vanilla Syrup Pump Bottle'),
('SYR-9002', 'Caramel Syrup Pump Bottle'),
('SYR-9003', 'Hazelnut Syrup Pump Bottle'),
('SYR-9004', 'Dutch Cocoa Powder 1kg'),
('FUR-1101', 'Cafe Menu Chalkboard A Frame'),
('FUR-1102', 'Barista Canvas Apron Heavy'),
('FUR-1103', 'Order Number Table Stands'),
('FUR-1104', 'Bar Stool Walnut 65cm'),
('FUR-1105', 'Teak Serving Tray Large');

\echo ''
\echo '### Đã nạp 50 mặt hàng.'
\echo ''


-- =====================================================================
-- PHẦN 1 — Trigram là gì
-- =====================================================================

\echo '=== 1.1  Một chuỗi bị băm thành các mẩu 3 ký tự ==='
SELECT show_trgm('coffee') AS trigrams_coffee,
       show_trgm('cofee')  AS trigrams_typo;

\echo '--> Postgres viết thường chuỗi, đệm khoảng trắng hai đầu, rồi trượt cửa sổ 3'
\echo '    ký tự (kết quả in ra đã được sắp xếp, không theo thứ tự trong chuỗi).'
\echo '    "coffee" ra 7 mẩu, "cofee" ra 6 — và 5 mẩu trong đó là CHUNG.'
\echo '    Đây là toàn bộ mẹo của pg_trgm: đo độ chồng lấn giữa hai tập trigram.'
\echo ''

\echo '=== 1.2  similarity() = số trigram chung / số trigram hợp ==='
SELECT
    similarity('coffee', 'coffee')  AS exact,
    similarity('coffee', 'cofee')   AS typo_missing_letter,
    similarity('coffee', 'coffe')   AS typo_truncated,
    similarity('coffee', 'coffeee') AS typo_extra_letter,
    similarity('coffee', 'tea')     AS unrelated;

\echo '--> Giá trị 0..1. Không dính gì tới nghĩa của từ, thuần ký tự.'
\echo '    Đó là lý do nó bắt được lỗi gõ phím mà FTS chịu thua.'
\echo ''


-- =====================================================================
-- PHẦN 2 — Cứu LIKE '%...%' bằng GIN + gin_trgm_ops
--
-- Ở 001 phần 5.3 ta đã thấy: tắt seqscan rồi LIKE vẫn phải Seq Scan
-- vì không index nào dùng được. Giờ sửa điều đó.
-- =====================================================================

\echo '=== 2.1  Trước khi có index: bắt buộc Seq Scan ==='
ANALYZE catalog_items;
SET enable_seqscan = off;
EXPLAIN (COSTS OFF, ANALYZE, TIMING OFF, SUMMARY OFF)
SELECT id, product_name FROM catalog_items WHERE product_name ILIKE '%grinder%';
RESET enable_seqscan;
\echo ''

\echo '=== 2.2  Tạo GIN index với toán tử trigram ==='
CREATE INDEX idx_catalog_name_trgm
ON catalog_items
USING GIN (product_name gin_trgm_ops);

ANALYZE catalog_items;

SET enable_seqscan = off;
EXPLAIN (COSTS OFF, ANALYZE, TIMING OFF, SUMMARY OFF)
SELECT id, product_name FROM catalog_items WHERE product_name ILIKE '%grinder%';
RESET enable_seqscan;

\echo '--> Giờ đã có Bitmap Index Scan. gin_trgm_ops hỗ trợ LIKE, ILIKE, ~, ~*'
\echo '    kể cả khi pattern bắt đầu bằng %. Đây mới là câu trả lời đúng cho'
\echo '    "LIKE ''%coffee%'' chậm", chứ không phải chuyển sang full-text search.'
\echo ''

\echo '=== 2.3  Kết quả vẫn đúng như LIKE thường ==='
SELECT id, sku, product_name
FROM catalog_items
WHERE product_name ILIKE '%grinder%'
ORDER BY id;

\echo '--> Ngữ nghĩa không đổi một chút nào. Index chỉ làm nó nhanh lên.'
\echo '    Recheck Cond trong plan chính là bước Postgres lọc lại false positive'
\echo '    do trigram gây ra.'
\echo ''

\echo '=== 2.4  Giới hạn: pattern ngắn hơn 3 ký tự thì trigram bó tay ==='
SET enable_seqscan = off;
EXPLAIN (COSTS OFF, ANALYZE, TIMING OFF, SUMMARY OFF)
SELECT id FROM catalog_items WHERE product_name ILIKE '%ug%';
RESET enable_seqscan;

\echo '--> Đọc kỹ plan: Bitmap Index Scan trả về actual rows=50 (toàn bộ bảng),'
\echo '    rồi "Rows Removed by Index Recheck: 48". Index chạy nhưng lọc được 0 dòng,'
\echo '    vì pattern 2 ký tự không sinh ra trigram hoàn chỉnh nào để tra.'
\echo '    Nhớ điều này khi làm ô search cho phép gõ 1-2 ký tự — đặt độ dài tối'
\echo '    thiểu 3 ký tự trước khi bắn query.'
\echo ''


-- =====================================================================
-- PHẦN 3 — Tìm gần đúng: cái mà FTS hoàn toàn không làm được
-- =====================================================================

\echo '=== 3.1  Nhắc lại: FTS trả 0 dòng khi gõ sai ==='
SELECT count(*) AS fts_hits_for_typo
FROM catalog_items
WHERE to_tsvector('english', product_name) @@ plainto_tsquery('english', 'cofee grindr');
\echo ''

\echo '=== 3.2  Trigram vẫn tìm ra ==='
SELECT id, sku, product_name,
       round(similarity(product_name, 'cofee grindr')::numeric, 4) AS sim
FROM catalog_items
WHERE product_name % 'cofee grindr'
ORDER BY sim DESC;

\echo '--> Toán tử % nghĩa là "similarity vượt ngưỡng". Ngưỡng mặc định 0.3.'
\echo ''

\echo '=== 3.3  Ngưỡng điều chỉnh được, và nó đổi hoàn toàn kết quả ==='
SHOW pg_trgm.similarity_threshold;

SET pg_trgm.similarity_threshold = 0.15;
SELECT count(*) AS hits_at_0_15 FROM catalog_items WHERE product_name % 'cofee grindr';

SET pg_trgm.similarity_threshold = 0.5;
SELECT count(*) AS hits_at_0_50 FROM catalog_items WHERE product_name % 'cofee grindr';

RESET pg_trgm.similarity_threshold;

\echo '--> Ngưỡng thấp: nhiều rác. Ngưỡng cao: bỏ sót. Không có con số đúng'
\echo '    cho mọi trường hợp, phải chỉnh theo data thật của bạn.'
\echo ''

\echo '=== 3.4  word_similarity: khớp một TỪ nằm trong chuỗi dài ==='
SELECT
    similarity('grinder', 'Conical Burr Coffee Grinder')       AS plain_similarity,
    word_similarity('grinder', 'Conical Burr Coffee Grinder')  AS word_similarity;

\echo '--> similarity() bị chuỗi dài làm loãng: một từ đúng trong một tên dài'
\echo '    cho điểm rất thấp. word_similarity() chỉ so với đoạn khớp nhất bên trong'
\echo '    -> điểm cao. Search "1 từ khoá trong 1 tên sản phẩm dài" phải dùng cái này'
\echo '    (toán tử <%, index bằng gin_trgm_ops).'
\echo ''

\echo '=== 3.5  Dùng <% để tìm theo từ khoá lẻ ==='
SELECT id, sku, product_name,
       round(word_similarity('grindr', product_name)::numeric, 4) AS wsim
FROM catalog_items
WHERE 'grindr' <% product_name
ORDER BY wsim DESC;

\echo '--> Chú ý chiều: pattern <% chuỗi. Đảo chiều là ra kết quả khác.'
\echo ''


-- =====================================================================
-- PHẦN 4 — Top-N gần giống nhất: toán tử khoảng cách <-> + GiST
-- =====================================================================

\echo '=== 4.1  Xếp hạng theo độ khác biệt (1 - similarity) ==='
SELECT id, sku, product_name,
       round((product_name <-> 'expreso machin')::numeric, 4) AS distance
FROM catalog_items
ORDER BY product_name <-> 'expreso machin'
LIMIT 8;

\echo '--> Không có WHERE. Đây là "cho tôi 8 cái giống nhất", đúng bài toán'
\echo '    gợi ý "ý bạn là...?" khi search ra 0 kết quả.'
\echo ''

\echo '=== 4.2  ORDER BY <-> cần GiST, không phải GIN ==='
CREATE INDEX idx_catalog_name_trgm_gist
ON catalog_items
USING GIST (product_name gist_trgm_ops);

ANALYZE catalog_items;

SET enable_seqscan = off;
EXPLAIN (COSTS OFF, ANALYZE, TIMING OFF, SUMMARY OFF)
SELECT id, product_name
FROM catalog_items
ORDER BY product_name <-> 'expreso machin'
LIMIT 8;
RESET enable_seqscan;

\echo '--> "Index Scan using ...gist" nghĩa là index trả về sẵn thứ tự — không'
\echo '    cần Sort toàn bộ bảng. GIN KHÔNG làm được việc này.'
\echo ''
\echo '    GIN  vs  GiST cho trigram:'
\echo '      GIN  : tra cứu nhanh hơn, index to hơn, build/update chậm hơn,'
\echo '             KHÔNG hỗ trợ ORDER BY <->  -> dùng cho LIKE/ILIKE/%'
\echo '      GiST : hỗ trợ KNN ORDER BY <->, index nhỏ hơn, tra cứu chậm hơn'
\echo '             -> dùng khi cần top-N gần giống nhất'
\echo '    Cần cả hai kiểu query thì tạo cả hai index, chấp nhận tốn chỗ.'
\echo ''

DROP INDEX idx_catalog_name_trgm_gist;


-- =====================================================================
-- PHẦN 5 — Ghép FTS và trigram lại: pattern thật trong production
-- =====================================================================

\echo '=== 5.1  Đường chính: FTS (nhanh, đúng ngữ nghĩa) ==='
SELECT id, sku, product_name
FROM catalog_items
WHERE to_tsvector('english', product_name) @@ websearch_to_tsquery('english', 'coffee grinder')
ORDER BY id;
\echo ''

\echo '=== 5.2  Đường dự phòng: FTS ra rỗng thì rơi xuống trigram ==='
WITH fts AS (
    SELECT id, sku, product_name, 'fts' AS matched_by
    FROM catalog_items
    WHERE to_tsvector('english', product_name)
          @@ websearch_to_tsquery('english', 'cofee grindr')
),
fuzzy AS (
    SELECT id, sku, product_name, 'trigram' AS matched_by
    FROM catalog_items
    WHERE product_name % 'cofee grindr'
    ORDER BY similarity(product_name, 'cofee grindr') DESC
    LIMIT 5
)
SELECT * FROM fts
UNION ALL
SELECT * FROM fuzzy WHERE NOT EXISTS (SELECT 1 FROM fts);

\echo '--> FTS chạy trước vì nó rẻ và chính xác. Chỉ khi nó ra rỗng mới trả tiền'
\echo '    cho trigram. Người dùng gõ đúng thì không phải chịu chi phí fuzzy.'
\echo ''

\echo '=== 5.3  Gõ đúng: nhánh trigram không bao giờ chạy ==='
WITH fts AS (
    SELECT id, sku, product_name, 'fts' AS matched_by
    FROM catalog_items
    WHERE to_tsvector('english', product_name)
          @@ websearch_to_tsquery('english', 'coffee grinder')
),
fuzzy AS (
    SELECT id, sku, product_name, 'trigram' AS matched_by
    FROM catalog_items
    WHERE product_name % 'coffee grinder'
    ORDER BY similarity(product_name, 'coffee grinder') DESC
    LIMIT 5
)
SELECT * FROM fts
UNION ALL
SELECT * FROM fuzzy WHERE NOT EXISTS (SELECT 1 FROM fts);

\echo '--> Nhánh trigram không chạy vì fts có kết quả.'
\echo ''

\echo '=== 5.4  Bảng chọn công cụ ==='
\echo ''
\echo '    Bài toán                                   Dùng gì'
\echo '    -----------------------------------------  ----------------------------'
\echo '    Tìm theo TỪ, hiểu số ít/số nhiều, chia thì  FTS: tsvector + GIN'
\echo '    Xếp hạng theo độ liên quan                 ts_rank / ts_rank_cd'
\echo '    Bôi đậm đoạn khớp                          ts_headline'
\echo '    LIKE ''%chuỗi con%'' phải nhanh              pg_trgm: GIN gin_trgm_ops'
\echo '    Gõ sai chính tả vẫn ra kết quả             pg_trgm: % / similarity()'
\echo '    "Ý bạn là...?" top-N gần giống nhất        pg_trgm: <-> + GiST'
\echo '    Autocomplete gõ tới đâu gợi ý tới đó       to_tsquery(''abc:*'') hoặc trigram'
\echo '    Khớp chính xác / khoảng / sắp xếp          B-tree'
\echo '    Prefix LIKE ''abc%''                         B-tree text_pattern_ops'
\echo ''
\echo '    FTS và trigram KHÔNG thay thế nhau. FTS hiểu TỪ, trigram hiểu KÝ TỰ.'
\echo '    Hệ thống search nghiêm túc thường dùng cả hai.'
\echo ''
\echo '### Xong 003.'
\echo ''
