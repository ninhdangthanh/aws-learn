-- =====================================================================
-- 001-core-fts.sql — Full-Text Search cơ bản trong PostgreSQL
--
--   Chạy:  psql -d <db> -f sql/001-core-fts.sql
--
--   Demo này KHÔNG đo tốc độ. 50 row thì Postgres luôn seq-scan.
--   Demo này chứng minh HÀNH VI: LIKE và FTS match ra hai tập kết quả
--   khác nhau, và khác ở đâu.
-- =====================================================================

\set ON_ERROR_STOP on
\pset pager off

DROP TABLE IF EXISTS products;

CREATE TABLE products (
    id          BIGSERIAL PRIMARY KEY,
    name        TEXT NOT NULL,
    description TEXT NOT NULL
);

INSERT INTO products (name, description) VALUES
('Professional Espresso Machine',    'Commercial grade machine for restaurants and cafes. Brews up to 200 shots per hour.'),
('Compact Coffee Grinder',           'Burr grinder with 40 grind settings for espresso and filter coffee.'),
('Pour Over Kettle',                 'Gooseneck kettle with precise temperature control for manual brewing.'),
('French Press 1L',                  'Classic press that brewed rich coffee without any paper filters.'),
('Cold Brew Carafe',                 'Steep coarse grounds overnight, serve chilled the next morning.'),
('Electric Milk Frother',            'Creates dense microfoam without a steam wand.'),
('Green Tea Sampler',                'Twelve loose leaf green tea varieties from Japan and China.'),
('Black Tea Tin',                    'Strong Assam black tea leaves, the tin holds 250 grams.'),
('Herbal Tea Infuser',               'Stainless steel infuser for herbal tea and flowering blends.'),
('Teak Serving Tray',                'Solid teak tray with raised edges, use it instead of plastic.'),
('Handheld Steam Cleaner',           'Portable steam cleaner for kitchen surfaces and tile grout.'),
('Food Steamer Basket',              'Collapsible steamer basket, use it instead of a boiling pot.'),
('Espresso Tamper 58mm',             'Flat base tamper machined from a single block of stainless steel.'),
('Drip Coffee Maker 12 Cup',         'Programmable machine that brews a full pot in eight minutes.'),
('Single Serve Pod System',          'Compact machine for coffee pods, ready in thirty seconds.'),
('Coffeehouse Blend 1kg',            'Whole bean blend roasted for coffeehouse style drinks.'),
('Decaffeinated Beans 500g',         'Swiss water process decaf, no chemical solvents involved.'),
('Reusable Coffee Filter',           'Stainless mesh filter that fits most drip machines.'),
('Digital Kitchen Scale',            'Zero point one gram precision for weighing coffee doses.'),
('Milk Jug 600ml',                   'Polished pitcher for latte art and steaming milk.'),
('Knock Box Drawer',                 'Under counter drawer that collects used pucks.'),
('Bottomless Portafilter',           'Naked portafilter reveals extraction flaws while brewing.'),
('Water Filter Cartridge',           'Reduces scale buildup inside espresso machines.'),
('Descaling Solution 500ml',         'Removes limescale from any coffee machine safely.'),
('Ceramic Mug Set of 4',             'Thick walled mugs that keep drinks hot for longer.'),
('Double Wall Glasses',              'Borosilicate glasses for espresso and tea service.'),
('Vacuum Insulated Flask',           'Keeps brewed coffee hot for twelve hours.'),
('Travel Tumbler 350ml',             'Leakproof tumbler sized for a single cappuccino.'),
('Siphon Brewer Kit',                'Vacuum siphon that brews coffee over a halogen lamp.'),
('Immersion Brewer 250ml',           'Portable brewer, coffee brewed in ninety seconds.'),
('Moka Pot 6 Cup',                   'Aluminium stovetop pot, strong coffee without a machine.'),
('Induction Cooktop Single',         'Portable induction hob for stovetop brewers.'),
('Machine Cleaning Tablets',         'Backflush tablets for every commercial machine.'),
('Group Head Brush',                 'Nylon brush for cleaning the group head daily.'),
('Bean Storage Canister',            'Airtight canister with a one way valve for beans.'),
('Precision Dosing Cup',             'Aluminium cup that catches grounds from the grinder.'),
('Latte Art Stencils',               'Twelve stencils for dusting cocoa over foam.'),
('Barista Apron Canvas',             'Heavy canvas apron with a towel loop and two pockets.'),
('Cupping Bowl Set',                 'Six porcelain bowls for professional coffee cupping.'),
('Digital Refractometer',            'Measures total dissolved solids in brewed coffee.'),
('Stovetop Tea Kettle',              'Whistling kettle, boils water for tea in six minutes.'),
('Bamboo Matcha Whisk',              'Traditional chasen whisk for matcha green tea.'),
('Instant Coffee Jar',               'Freeze dried granules, no machine required at all.'),
('Chocolate Powder 1kg',             'Dutch processed cocoa for mochas and hot chocolate.'),
('Syrup Pump Bottles',               'Three pumps for vanilla, caramel and hazelnut syrup.'),
('Steel Sugar Dispenser',            'Dispenser that meters one spoon at a time.'),
('Cafe Menu Chalkboard',             'A frame chalkboard for the pavement outside.'),
('Order Number Stands',              'Numbered table stands, set of twenty.'),
('Espresso Machine Gasket',          'Silicone group gasket, a spare part for repairs.'),
('Clip On Thermometer',              'Dial thermometer clips onto a milk jug instead of guessing.');

\echo ''
\echo '### Đã nạp 50 sản phẩm.'
\echo ''


-- =====================================================================
-- PHẦN 1 — LIKE sai ở đâu
--
-- Không phải "LIKE chậm". Vấn đề là LIKE so khớp CHUỖI KÝ TỰ,
-- còn người dùng gõ vào TỪ. Hai thứ đó không giống nhau.
-- =====================================================================

\echo '=== 1.1  LIKE trả về rác: tìm "tea" ra cả steam / instead / teak ==='
SELECT id, name, description
FROM products
WHERE name ILIKE '%tea%' OR description ILIKE '%tea%'
ORDER BY id;

\echo '--> 12 dòng, mà chỉ 6 dòng thật sự nói về trà. Phần còn lại dính vì "tea"'
\echo '    nằm bên trong steam / steamer / steaming / instead / teak.'
\echo ''

\echo '=== 1.2  LIKE bỏ sót: tìm "machines" không ra "machine" ==='
SELECT id, name
FROM products
WHERE name ILIKE '%machines%' OR description ILIKE '%machines%'
ORDER BY id;

\echo '--> Chỉ 2 dòng. Thêm một chữ "s" là mất sạch các sản phẩm dùng'
\echo '    số ít "machine". Người dùng không có nghĩa vụ gõ đúng số ít/số nhiều.'
\echo ''

\echo '=== 1.3  LIKE phụ thuộc thứ tự từ: "coffee machine" ==='
SELECT id, name, description
FROM products
WHERE (name || ' ' || description) ILIKE '%coffee machine%'
ORDER BY id;

\echo '--> Chỉ ra dòng nào viết đúng y hệt "coffee machine" cạnh nhau.'
\echo '    "machine for coffee pods" hay "coffee without a machine" đều trượt.'
\echo ''


-- =====================================================================
-- PHẦN 2 — tsvector: Postgres nhìn thấy gì
--
-- to_tsvector() làm 3 việc: tách từ (tokenize), bỏ stop word,
-- và rút gọn từ về gốc (stemming). Kết quả là danh sách LEXEME + vị trí.
-- =====================================================================

\echo '=== 2.1  Một câu biến thành tsvector như thế nào ==='
SELECT to_tsvector('english', 'Professional coffee machine for restaurants') AS tsv;

\echo '--> ''coffe'':2 ''machin'':3 ''profession'':1 ''restaur'':5'
\echo '    "for" biến mất (stop word). "machine" -> "machin". Số là vị trí từ.'
\echo ''

\echo '=== 2.2  Các biến thể của cùng một từ cho ra CÙNG một lexeme ==='
SELECT
    to_tsvector('english', 'machine')  AS m1,
    to_tsvector('english', 'machines') AS m2,
    to_tsvector('english', 'machined') AS m3,
    to_tsvector('english', 'brewing')  AS b1,
    to_tsvector('english', 'brewed')   AS b2,
    to_tsvector('english', 'brews')    AS b3;

\echo '--> Đây chính là thứ LIKE không có: machine/machines/machined -> machin.'
\echo '    Lưu ý "machined" (gia công) cũng về machin — stemming có thể quá tay.'
\echo ''

\echo '=== 2.3  Câu truy vấn của người dùng biến thành tsquery ==='
SELECT
    plainto_tsquery('english', 'coffee machine')     AS plain,
    phraseto_tsquery('english', 'coffee machine')    AS phrase,
    websearch_to_tsquery('english', 'coffee -tea')   AS websearch,
    to_tsquery('english', 'coffee & !tea')           AS raw;

\echo '--> plainto  : nối mọi từ bằng AND        -> ''coffe'' & ''machin'''
\echo '    phraseto : bắt buộc đứng liền kề      -> ''coffe'' <-> ''machin'''
\echo '    websearch: hiểu cú pháp người dùng gõ (dấu ngoặc kép, dấu trừ)'
\echo '    to_tsquery: cú pháp thô, ném lỗi nếu input bẩn -> ĐỪNG đưa input'
\echo '    người dùng thẳng vào đây. Dùng websearch_to_tsquery.'
\echo ''


-- =====================================================================
-- PHẦN 3 — FTS sửa cả 3 lỗi ở phần 1
-- =====================================================================

\echo '=== 3.1  "tea" — không còn steam/instead/teak ==='
SELECT id, name
FROM products
WHERE to_tsvector('english', name || ' ' || description)
      @@ plainto_tsquery('english', 'tea')
ORDER BY id;

\echo '--> Đúng 6 dòng, sạch rác. Vì "steam" tách ra là lexeme steam, không phải tea.'
\echo ''

\echo '=== 3.2  "machines" — giờ ra cả machine số ít ==='
SELECT id, name
FROM products
WHERE to_tsvector('english', name || ' ' || description)
      @@ plainto_tsquery('english', 'machines')
ORDER BY id;
\echo ''

\echo '=== 3.3  "coffee machine" — không quan tâm thứ tự từ ==='
SELECT id, name, description
FROM products
WHERE to_tsvector('english', name || ' ' || description)
      @@ plainto_tsquery('english', 'coffee machine')
ORDER BY id;

\echo '--> Bắt được cả "machine for coffee pods" lẫn "coffee without a machine".'
\echo '    Muốn ép đứng liền nhau thì đổi sang phraseto_tsquery.'
\echo ''

\echo '=== 3.4  phraseto_tsquery: ép "coffee machine" phải liền kề ==='
SELECT id, name, description
FROM products
WHERE to_tsvector('english', name || ' ' || description)
      @@ phraseto_tsquery('english', 'coffee machine')
ORDER BY id;
\echo ''


-- =====================================================================
-- PHẦN 4 — Index: vì sao BẮT BUỘC phải truyền 'english'
-- =====================================================================

\echo '=== 4.1  Bản to_tsvector 1 tham số KHÔNG index được ==='
-- Thử tạo index bằng bản 1 tham số và bắt lỗi lại, để script không dừng.
-- (Ghi vào bảng tạm rồi SELECT ra, thay vì RAISE NOTICE — NOTICE đi qua stderr
--  nên khi bạn pipe output ra file nó sẽ chen sai vị trí.)
CREATE TEMP TABLE immutable_probe (result TEXT);

DO $$
BEGIN
    EXECUTE 'CREATE INDEX bad_idx ON products USING GIN (to_tsvector(description))';
    INSERT INTO immutable_probe VALUES ('Bất ngờ: index tạo được (không nên xảy ra)');
EXCEPTION WHEN others THEN
    INSERT INTO immutable_probe VALUES ('Postgres từ chối, đúng như dự đoán -> ' || SQLERRM);
END $$;

SELECT result FROM immutable_probe;
DROP TABLE immutable_probe;

\echo '--> to_tsvector(text) 1 tham số dùng biến default_text_search_config,'
\echo '    mà biến đó đổi được lúc runtime -> hàm chỉ STABLE, không IMMUTABLE.'
\echo '    Index không được phép chứa hàm có thể đổi kết quả. Truyền'
\echo '    to_tsvector(''english'', ...) thì hàm mới IMMUTABLE và index được.'
\echo ''

\echo '=== 4.2  Cách cũ: expression index ==='
CREATE INDEX idx_products_fts_expr
ON products
USING GIN (to_tsvector('english', coalesce(name,'') || ' ' || coalesce(description,'')));

\echo '--> Chạy được, nhưng cái bẫy là: query phải lặp lại BIỂU THỨC Y HỆT.'
\echo '    Lệch một chữ coalesce, đổi thứ tự cột, thiếu dấu cách -> index không'
\echo '    được dùng và không ai báo cho bạn biết. Query im lặng chậm đi.'
\echo ''

\echo '=== 4.3  Cách nên dùng (PG12+): generated column STORED ==='
DROP INDEX idx_products_fts_expr;

ALTER TABLE products
ADD COLUMN search_doc tsvector
GENERATED ALWAYS AS (
    to_tsvector('english', coalesce(name,'') || ' ' || coalesce(description,''))
) STORED;

CREATE INDEX idx_products_search_doc ON products USING GIN (search_doc);

ANALYZE products;

SELECT id, name, search_doc FROM products WHERE id IN (1, 15) ORDER BY id;

\echo '--> Biểu thức nằm trong định nghĩa bảng, viết một lần duy nhất.'
\echo '    Postgres tự cập nhật khi name/description đổi. Query chỉ còn:'
\echo '        WHERE search_doc @@ websearch_to_tsquery(''english'', $1)'
\echo '    Đổi lại: tốn thêm dung lượng đĩa cho cột tsvector.'
\echo ''

\echo '=== 4.4  Generated column tự cập nhật khi UPDATE ==='
UPDATE products SET description = 'Now a herbal tea infuser as well.' WHERE id = 21;
SELECT id, name, search_doc FROM products WHERE id = 21;
\echo ''


-- =====================================================================
-- PHẦN 5 — Chứng minh GIN index thực sự được dùng
--
-- 50 row thì seq-scan luôn rẻ hơn, planner sẽ bỏ qua index.
-- Tắt seqscan để ép planner cho xem nó CÓ THỂ dùng index.
-- =====================================================================

\echo '=== 5.1  Mặc định: planner chọn Seq Scan (đúng, vì bảng bé) ==='
EXPLAIN (ANALYZE, COSTS OFF, TIMING OFF, SUMMARY OFF)
SELECT id, name FROM products
WHERE search_doc @@ websearch_to_tsquery('english', 'coffee machine');
\echo ''

\echo '=== 5.2  Ép tắt seqscan: xuất hiện Bitmap Index Scan trên GIN ==='
SET enable_seqscan = off;
EXPLAIN (ANALYZE, COSTS OFF, TIMING OFF, SUMMARY OFF)
SELECT id, name FROM products
WHERE search_doc @@ websearch_to_tsquery('english', 'coffee machine');
RESET enable_seqscan;

\echo '--> Thấy "Bitmap Index Scan on idx_products_search_doc" nghĩa là index'
\echo '    dùng được. Trên bảng vài triệu row, planner sẽ tự chọn nó.'
\echo '    Đây là cách kiểm tra index đúng đắn khi dev trên data nhỏ.'
\echo ''

\echo '=== 5.3  Đối chứng: LIKE %...% thì kể cả tắt seqscan vẫn phải Seq Scan ==='
SET enable_seqscan = off;
EXPLAIN (ANALYZE, COSTS OFF, TIMING OFF, SUMMARY OFF)
SELECT id, name FROM products WHERE description ILIKE '%machine%';
RESET enable_seqscan;

\echo '--> Không có index nào cứu được. B-tree vô dụng với wildcard đầu chuỗi.'
\echo '    Muốn cứu nó thì cần pg_trgm — xem 003-trigram.sql.'
\echo ''


-- =====================================================================
-- PHẦN 6 — Giới hạn của FTS (chỗ idea gốc nhầm)
--
-- FTS KHÔNG phải bản thay thế của LIKE '%...%'.
-- FTS khớp TỪ ĐẦY ĐỦ sau khi stem. Nó không tìm chuỗi con.
-- =====================================================================

\echo '=== 6.1  Gõ thiếu chữ: "coff" — FTS trả về 0 dòng ==='
SELECT count(*) AS fts_hits
FROM products
WHERE search_doc @@ plainto_tsquery('english', 'coff');

SELECT count(*) AS like_hits
FROM products
WHERE description ILIKE '%coff%' OR name ILIKE '%coff%';

\echo '--> FTS: 0. LIKE: nhiều. Vì "coff" không phải một lexeme nào cả.'
\echo '    Autocomplete / search-as-you-type KHÔNG dùng plainto_tsquery được.'
\echo ''

\echo '=== 6.2  Cách vá cho prefix: toán tử :* trong to_tsquery ==='
SELECT id, name
FROM products
WHERE search_doc @@ to_tsquery('english', 'coff:*')
ORDER BY id
LIMIT 10;

\echo '--> :* là prefix match trên lexeme, GIN vẫn dùng được. Đủ cho autocomplete.'
\echo '    Nhưng vẫn chỉ khớp ĐẦU từ: "ffee:*" vẫn ra 0 dòng.'
\echo ''

\echo '=== 6.3  Gõ sai chính tả: "cofee" — FTS bó tay hoàn toàn ==='
SELECT count(*) AS fts_hits
FROM products
WHERE search_doc @@ plainto_tsquery('english', 'cofee machine');

\echo '--> 0 dòng. FTS không có khái niệm "gần giống".'
\echo '    Bài toán typo thuộc về pg_trgm — xem 003-trigram.sql.'
\echo ''
\echo '### Xong 001. Tiếp theo: 002-ranking.sql (xếp hạng kết quả).'
\echo ''
