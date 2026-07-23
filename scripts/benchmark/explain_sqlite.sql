-- SQLite EXPLAIN for the cross-database comparison (C4/C6).
-- Run inside a one-off alpine+sqlite container against the benchmark DB.
.mode column
.headers on
SELECT '--- EXPLAIN QUERY PLAN (list query) ---' AS section;
EXPLAIN QUERY PLAN
SELECT id,title,slug,excerpt,author_id,status,post_type,format,visibility,locale,is_pinned,published_at,created_at,updated_at
FROM articles
WHERE status='published' AND post_type='post'
ORDER BY is_pinned DESC, published_at DESC, created_at DESC
LIMIT 20 OFFSET 0;

SELECT '--- indexes on articles ---' AS section;
SELECT name FROM sqlite_master WHERE type='index' AND tbl_name='articles';

SELECT '--- article count ---' AS section;
SELECT COUNT(*) AS total FROM articles;
