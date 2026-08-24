-- =====================================================================
-- 002-ranking.sql — Xếp hạng kết quả: setweight, ts_rank, ts_headline
--
--   Chạy:  psql -d <db> -f sql/002-ranking.sql
--
--   001 trả lời "dòng nào KHỚP". File này trả lời "dòng nào KHỚP NHẤT".
--   Một search engine trả về 200 dòng không sắp xếp thì cũng vô dụng
--   như trả về 0 dòng.
-- =====================================================================

\set ON_ERROR_STOP on
\pset pager off

DROP TABLE IF EXISTS articles;

CREATE TABLE articles (
    id    BIGSERIAL PRIMARY KEY,
    title TEXT NOT NULL,
    body  TEXT NOT NULL
);

INSERT INTO articles (title, body) VALUES
('Postgres Index Basics',              'An index is a data structure that speeds up lookups. Postgres ships with btree, hash, gin and gist types. Picking the right index matters more than adding many.'),
('When B-tree Is Enough',              'Most queries need nothing more than a btree index on the filtered column. Reach for exotic types only after you have measured.'),
('Understanding GIN Indexes',          'A GIN index stores one entry per token instead of one per row. That makes it the right choice for arrays, jsonb and tsvector columns.'),
('Query Planner Explained',            'The planner estimates a cost for every access path and picks the cheapest one. Statistics gathered by ANALYZE drive those estimates.'),
('Vacuum and Table Bloat',             'Dead tuples pile up after every update. Autovacuum reclaims that space, and a bloated table makes every scan slower than it should be.'),
('Connection Pooling with PgBouncer',  'Postgres forks a process per connection. A pooler keeps that count low so memory usage stays predictable under load.'),
('Transaction Isolation Levels',       'Read committed is the default. Repeatable read and serializable trade throughput for stronger guarantees.'),
('Deadlocks in Production',            'Two transactions grab the same rows in a different order and both wait forever. Postgres detects the cycle and kills one of them.'),
('Partial Index Tricks',               'A partial index covers only rows matching a predicate. When ninety percent of rows are archived, the index shrinks tenfold.'),
('Covering Index and INCLUDE',         'An index with INCLUDE columns can answer a query without touching the heap at all. That is an index only scan.'),
('Streaming Replication Setup',        'Configure the primary, take a base backup, then start the standby. That is the whole procedure.'),
('High Availability Playbook',         'Availability depends on replication lag, failover automation and connection routing. Monitor replication slots, watch replication lag on every standby, and rehearse failover before you actually need it. Logical replication also helps during major version upgrades.'),
('Logical Decoding Primer',            'Logical decoding turns the write ahead log into a stream of row changes that downstream consumers can read.'),
('WAL and Checkpoints',                'The write ahead log makes crash recovery possible. Checkpoints flush dirty buffers and let old segments be recycled.'),
('Choosing a Primary Key',             'Sequential integers pack better in a btree index than random uuids. Use uuid v7 when you need both order and global uniqueness.'),
('JSONB Storage Internals',            'JSONB parses the document once and stores a binary tree. That is why a jsonb index can answer containment queries.'),
('Array Columns in Practice',          'An array column is convenient but hard to join against. A GIN index makes containment lookups on it fast enough.'),
('Full Text Search Overview',          'The tsvector type holds lexemes, the tsquery type holds a search expression, and a GIN index connects the two.'),
('Trigram Search Explained',           'Trigrams break a string into three character chunks so that similarity can be measured between misspelled words.'),
('Index Only Scans',                   'When every column a query needs lives in the index, Postgres can skip the heap. The visibility map decides whether that is safe.'),
('Bitmap Heap Scan Demystified',       'A bitmap scan collects matching tuple pointers first, sorts them, then reads each heap page a single time.'),
('Statistics and n_distinct',          'A wrong n_distinct estimate sends the planner down the wrong path. Extended statistics fix correlated columns.'),
('Table Partitioning Guide',           'Declarative partitioning splits one huge table into children. Pruning skips partitions the query cannot match.'),
('Foreign Data Wrappers',              'A foreign table looks local but reads from another server. Push down as much of the filter as you can.'),
('Advisory Locks for Jobs',            'An advisory lock is cheap, application defined, and released when the session ends. Perfect for single runner jobs.'),
('Row Level Security Basics',          'A policy is an implicit WHERE clause the database always appends. It cannot be forgotten by an application developer.'),
('Upsert with ON CONFLICT',            'ON CONFLICT DO UPDATE needs a unique index to detect the conflict. Without one the statement simply fails.'),
('Generated Columns Explained',        'A stored generated column is computed on write and behaves like a normal column for reads and for indexing.'),
('Materialized View Refresh',          'A materialized view never refreshes itself. Something outside the database has to call REFRESH on a schedule.'),
('Common Table Expressions',           'Since version twelve a CTE can be inlined by the planner. Add MATERIALIZED when you want the old fence behaviour.'),
('Window Functions Cookbook',          'Window functions compute across a frame of rows without collapsing them. Running totals and rankings become one pass.'),
('LATERAL Joins by Example',           'A LATERAL subquery can reference columns from the left side. It is the clean way to fetch top n per group.'),
('Skip Locked Queues',                 'FOR UPDATE SKIP LOCKED turns a plain table into a work queue that many workers can drain safely.'),
('Timeouts Every Service Needs',       'Set statement_timeout, lock_timeout and idle_in_transaction_session_timeout. A query with no timeout is an outage waiting.'),
('Reading EXPLAIN ANALYZE',            'Compare estimated rows against actual rows. A large gap means the planner was misled by stale statistics.'),
('Bloat Detection Queries',            'pgstattuple reports real bloat but scans the table. The estimate query is cheaper and usually good enough.'),
('Autovacuum Tuning',                  'Lower the scale factor on large tables. The default ratio means a billion row table waits far too long.'),
('Backup Strategy Basics',             'A backup you have never restored is a rumour. Practise a full restore on a schedule, not during an incident.'),
('Point In Time Recovery',             'Archive the write ahead log and you can restore to any second. The base backup alone is not enough.'),
('Schema Migration Safety',            'Add a column with a default is instant now, but a new index still locks writes unless you build it concurrently.'),
('Zero Downtime Column Drop',          'Drop the column from application code first, deploy, and only then drop it from the database.'),
('The Complete Database Performance Guide', 'Performance work starts with measurement, never with guessing. Collect timing from the application, not only from the database. Look at the slowest endpoint and follow it down through the service layer into the query layer. Most slow endpoints are slow because of the number of round trips, not because of a single slow statement. Once you have found a genuinely slow statement, read its plan, check whether an index would help, and confirm the fix with a before and after measurement. Only then move to the next one. Repeat this loop until the endpoint is fast enough, and resist the urge to tune settings you have not measured.'),
('Caching Layers Compared',            'An application cache is fast but goes stale. A materialized view is slower but always consistent within its refresh window.'),
('Redis or Postgres for Queues',       'Postgres gives you transactions with the rest of your data. Redis gives you throughput. Pick the one that matches your failure mode.'),
('Sharding Decision Framework',        'Shard only when a single writer is genuinely saturated. Every alternative is cheaper than living with cross shard queries.'),
('Read Replicas and Staleness',        'A read replica lags. Route reads that tolerate staleness to it, and keep read your own writes traffic on the primary.'),
('Postgres Extensions Worth Knowing',  'pg_stat_statements, pg_trgm, pgcrypto and postgis cover most needs without leaving the database.'),
('pg_stat_statements Deep Dive',       'It aggregates by normalized query text. Sort by total time, not by mean time, to find what actually costs you.'),
('Locking Basics for Developers',      'A plain SELECT takes no lock that blocks writers. ALTER TABLE takes one that blocks everything, briefly.'),
('Postgres Upgrade Checklist',         'Test the new planner on a copy of production traffic. A version upgrade can change plans in both directions.');

\echo ''
\echo '### Đã nạp 50 bài viết.'
\echo ''


-- =====================================================================
-- PHẦN 1 — Không rank thì kết quả vô nghĩa
-- =====================================================================

\echo '=== 1.1  Tìm "index" mà không xếp hạng ==='
SELECT id, title
FROM articles
WHERE to_tsvector('english', title || ' ' || body) @@ plainto_tsquery('english', 'index')
ORDER BY id;

\echo '--> Ra một đống dòng theo thứ tự id. Bài đúng trọng tâm nhất nằm ở đâu?'
\echo '    Không ai biết. Đây là lý do phải có ts_rank.'
\echo ''


-- =====================================================================
-- PHẦN 2 — setweight: title phải nặng hơn body
-- =====================================================================

\echo '=== 2.1  Chuẩn bị cột tsvector CÓ TRỌNG SỐ ==='
ALTER TABLE articles
ADD COLUMN search_doc tsvector
GENERATED ALWAYS AS (
    setweight(to_tsvector('english', coalesce(title, '')), 'A') ||
    setweight(to_tsvector('english', coalesce(body,  '')), 'B')
) STORED;

CREATE INDEX idx_articles_search_doc ON articles USING GIN (search_doc);
ANALYZE articles;

SELECT id, title, search_doc FROM articles WHERE id = 11;

\echo '--> Chú ý hậu tố A và B sau vị trí: ''replic'':2A nghĩa là lexeme này'
\echo '    xuất hiện ở vị trí 3 với trọng số A. Postgres có 4 hạng: A > B > C > D.'
\echo '    Mặc định mọi lexeme là D.'
\echo ''

\echo '=== 2.2  Không trọng số: bài nhắc "replication" nhiều lần thắng ==='
SELECT
    id,
    title,
    round(ts_rank(to_tsvector('english', title || ' ' || body),
                  plainto_tsquery('english', 'replication'))::numeric, 5) AS rank_no_weight
FROM articles
WHERE to_tsvector('english', title || ' ' || body) @@ plainto_tsquery('english', 'replication')
ORDER BY rank_no_weight DESC;

\echo '--> "High Availability Playbook" leo lên đầu chỉ vì nó lặp từ 4 lần,'
\echo '    trong khi "Streaming Replication Setup" mới đúng là bài về chủ đề đó.'
\echo ''

\echo '=== 2.3  Có trọng số: title thắng ==='
SELECT
    id,
    title,
    round(ts_rank(search_doc, plainto_tsquery('english', 'replication'))::numeric, 5) AS rank_weighted
FROM articles
WHERE search_doc @@ plainto_tsquery('english', 'replication')
ORDER BY rank_weighted DESC;

\echo '--> Thứ tự đảo lại. Từ khoá nằm ở title đáng giá hơn nằm ở body,'
\echo '    đúng với trực giác của người dùng.'
\echo ''

\echo '=== 2.4  Tự chỉnh mức chênh lệch bằng mảng trọng số ==='
SELECT
    id,
    title,
    round(ts_rank('{0.1, 0.2, 0.4, 1.0}'::float4[], search_doc,
                  plainto_tsquery('english', 'replication'))::numeric, 5) AS rank_default,
    round(ts_rank('{0.02, 0.05, 0.1, 1.0}'::float4[], search_doc,
                  plainto_tsquery('english', 'replication'))::numeric, 5) AS rank_title_heavy
FROM articles
WHERE search_doc @@ plainto_tsquery('english', 'replication')
ORDER BY rank_title_heavy DESC;

\echo '--> Mảng xếp theo thứ tự {D, C, B, A} — NGƯỢC bảng chữ cái, rất dễ nhầm.'
\echo '    Phần tử cuối là trọng số hạng A. Mặc định {0.1, 0.2, 0.4, 1.0}.'
\echo '    Mỗi giá trị PHẢI nằm trong [0,1] — vượt ra là lỗi "weight out of range".'
\echo '    Muốn title nặng hơn thì DÌM D/C/B xuống, chứ không đẩy A lên.'
\echo ''


-- =====================================================================
-- PHẦN 3 — Bài dài tự nhiên có rank cao. Đó là bug, không phải feature.
-- =====================================================================

\echo '=== 3.1  Bài dài và bài ngắn có CÙNG điểm khi không chuẩn hoá ==='
SELECT
    id,
    left(title, 42) AS title,
    length(body) AS body_len,
    round(ts_rank(search_doc, plainto_tsquery('english', 'index'), 0)::numeric, 5) AS norm_0_off,
    round(ts_rank(search_doc, plainto_tsquery('english', 'index'), 1)::numeric, 5) AS norm_1_log_len,
    round(ts_rank(search_doc, plainto_tsquery('english', 'index'), 2)::numeric, 5) AS norm_2_len
FROM articles
WHERE id IN (18, 42)
ORDER BY id;

\echo '--> Hai bài có norm_0 y hệt nhau (0.24317) dù bài 42 dài gấp 6 lần và'
\echo '    chỉ nhắc "index" đúng một lần trong 629 ký tự. Bật cờ 1 hoặc 2 thì'
\echo '    bài dài bị dìm xuống đúng như trực giác.'
\echo ''

\echo '=== 3.2  Toàn bộ kết quả, xếp theo bản CÓ chuẩn hoá độ dài ==='
SELECT
    id,
    left(title, 42) AS title,
    length(body) AS body_len,
    round(ts_rank(search_doc, plainto_tsquery('english', 'index'), 0)::numeric, 5) AS norm_0_off,
    round(ts_rank(search_doc, plainto_tsquery('english', 'index'), 1)::numeric, 5) AS norm_1_log_len
FROM articles
WHERE search_doc @@ plainto_tsquery('english', 'index')
ORDER BY norm_1_log_len DESC;

\echo '--> Tham số thứ 3 là bitmask chuẩn hoá, cộng dồn được (0|1|2|4|8|16|32):'
\echo '     0  không chuẩn hoá (mặc định)'
\echo '     1  chia cho 1 + log(độ dài tài liệu)'
\echo '     2  chia cho độ dài tài liệu'
\echo '     4  chia cho harmonic mean khoảng cách giữa các từ (chỉ ts_rank_cd)'
\echo '     8  chia cho số từ KHÁC NHAU trong tài liệu'
\echo '    16  chia cho 1 + log(số từ khác nhau)'
\echo '    32  rank / (rank + 1) -> ép về khoảng (0,1), tiện để hiển thị %'
\echo '    Thực tế hay dùng: ts_rank(doc, q, 1) hoặc ts_rank(doc, q, 32).'
\echo ''

\echo '=== 3.3  ts_rank_cd — thưởng cho các từ khoá đứng GẦN nhau ==='
SELECT
    round(ts_rank_cd(
        to_tsvector('english', 'postgres index tuning for busy teams'),
        plainto_tsquery('english', 'postgres index'))::numeric, 5) AS words_adjacent,
    round(ts_rank_cd(
        to_tsvector('english', 'postgres is a relational database that many teams run in production and it can build an index'),
        plainto_tsquery('english', 'postgres index'))::numeric, 5) AS words_far_apart;

\echo '--> Cùng khớp cả hai từ, nhưng tài liệu để chúng cạnh nhau được điểm cao'
\echo '    hơn hẳn. ts_rank chỉ đếm tần suất, ts_rank_cd (cover density) đo cả'
\echo '    KHOẢNG CÁCH. Nó cần vị trí từ, nên đừng strip_tsvector() nếu định dùng.'
\echo ''

\echo '=== 3.4  So sánh trên data thật ==='
SELECT
    id,
    left(title, 42) AS title,
    round(ts_rank(search_doc,    plainto_tsquery('english', 'postgres index'))::numeric, 5) AS rank_freq,
    round(ts_rank_cd(search_doc, plainto_tsquery('english', 'postgres index'))::numeric, 5) AS rank_cover_density
FROM articles
WHERE search_doc @@ plainto_tsquery('english', 'postgres index')
ORDER BY rank_cover_density DESC;
\echo ''


-- =====================================================================
-- PHẦN 4 — ts_headline: bôi đậm chỗ khớp
-- =====================================================================

\echo '=== 4.1  Highlight mặc định ==='
SELECT
    id,
    ts_headline('english', body, plainto_tsquery('english', 'index'),
                'StartSel=<<, StopSel=>>, MaxWords=18, MinWords=8') AS snippet
FROM articles
WHERE search_doc @@ plainto_tsquery('english', 'index')
ORDER BY ts_rank(search_doc, plainto_tsquery('english', 'index')) DESC
LIMIT 5;

\echo '--> Trả về đúng đoạn văn chứa từ khoá, không phải cả bài.'
\echo ''

\echo '=== 4.2  Nhiều đoạn trích cho bài dài ==='
SELECT
    ts_headline('english', body, plainto_tsquery('english', 'measurement index'),
                'StartSel=[, StopSel=], MaxFragments=2, FragmentDelimiter= ... , MaxWords=14, MinWords=6') AS snippet
FROM articles
WHERE title = 'The Complete Database Performance Guide';
\echo ''

\echo '=== 4.3  CẢNH BÁO: ts_headline rất đắt, và index không giúp được gì ==='
\echo '--- không có ts_headline ---'
EXPLAIN (COSTS ON, TIMING OFF, ANALYZE, SUMMARY OFF)
SELECT id FROM articles
WHERE search_doc @@ plainto_tsquery('english', 'index');

\echo '--- có ts_headline trên mọi dòng khớp ---'
EXPLAIN (COSTS ON, TIMING OFF, ANALYZE, SUMMARY OFF)
SELECT id, ts_headline('english', body, plainto_tsquery('english', 'index'))
FROM articles
WHERE search_doc @@ plainto_tsquery('english', 'index');

\echo '--> So cột cost tổng của hai plan. ts_headline phải PARSE LẠI toàn bộ body'
\echo '    cho TỪNG dòng khớp — cột tsvector đã tính sẵn hoàn toàn không giúp gì,'
\echo '    vì highlight cần văn bản gốc chứ không cần lexeme.'
\echo '    Nguyên tắc: lọc + rank + LIMIT trước, ts_headline chỉ chạy trên đúng'
\echo '    10-20 dòng của trang hiện tại. Xem 4.4.'
\echo ''

\echo '=== 4.4  Pattern đúng cho production: lọc/rank trong subquery, headline ngoài ==='
SELECT
    id,
    title,
    round(rank::numeric, 5) AS rank,
    ts_headline('english', body, q, 'StartSel=<b>, StopSel=</b>, MaxWords=16, MinWords=6') AS snippet
FROM (
    SELECT id, title, body, q,
           ts_rank(search_doc, q) AS rank
    FROM articles, websearch_to_tsquery('english', 'postgres index') AS q
    WHERE search_doc @@ q
    ORDER BY rank DESC
    LIMIT 5
) top_hits;

\echo '--> GIN index lọc ra tập nhỏ -> ts_rank sắp xếp -> LIMIT -> chỉ 5 dòng'
\echo '    phải chịu chi phí ts_headline.'
\echo ''


-- =====================================================================
-- PHẦN 5 — Cái bẫy lớn nhất của ranking trong production
-- =====================================================================

\echo '=== 5.1  ts_rank KHÔNG index được ==='
SET enable_seqscan = off;
EXPLAIN (COSTS OFF)
SELECT id, title FROM articles
WHERE search_doc @@ websearch_to_tsquery('english', 'index')
ORDER BY ts_rank(search_doc, websearch_to_tsquery('english', 'index')) DESC
LIMIT 10;
RESET enable_seqscan;

\echo '--> Chú ý bước Sort nằm SAU Bitmap Index Scan. GIN index lọc được dòng,'
\echo '    nhưng điểm rank phải tính từng dòng rồi sort toàn bộ.'
\echo '    Nghĩa là: nếu WHERE khớp 2 triệu dòng thì phải chấm điểm cả 2 triệu'
\echo '    rồi mới LIMIT 10 được. LIMIT không cứu được gì.'
\echo ''
\echo '    Cách xử lý thực tế:'
\echo '      - Thêm điều kiện thu hẹp trước (tenant, category, khoảng thời gian).'
\echo '      - Query đủ chặt để tập khớp nhỏ (AND thay vì OR).'
\echo '      - Cắt bằng CTE: lấy 1000 dòng bất kỳ rồi mới rank trong đó,'
\echo '        chấp nhận rank gần đúng.'
\echo '      - Tập dữ liệu thật sự lớn thì đây là lúc cân nhắc Elasticsearch.'
\echo ''

\echo '=== 5.2  Cắt bớt trước khi rank (rank gần đúng, đổi lại giới hạn chi phí) ==='
WITH candidates AS (
    SELECT id, title, search_doc
    FROM articles
    WHERE search_doc @@ websearch_to_tsquery('english', 'index')
    LIMIT 1000
)
SELECT id, title,
       round(ts_rank(search_doc, websearch_to_tsquery('english', 'index'))::numeric, 5) AS rank
FROM candidates
ORDER BY rank DESC
LIMIT 5;

\echo '--> LIMIT 1000 đặt trần cho chi phí sort. Với 50 dòng thì kết quả y hệt,'
\echo '    với 2 triệu dòng thì đây là khác biệt giữa 20ms và 4 giây.'
\echo ''
\echo '### Xong 002. Tiếp theo: 003-trigram.sql (typo + substring).'
\echo ''
