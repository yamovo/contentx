-- SQLite benchmark seed: 10,000 published articles.
-- Row-for-row identical to seed_postgres_10000.sql so the cross-database
-- comparison uses the same dataset.
--
-- Usage (against the SQLite database file the app created via AutoMigrate):
--   sqlite3 contentx.db < seed_sqlite_10000.sql
--
-- Notes:
--   * SQLite has no REPEAT(); replace(hex(zeroblob(40)), '00', s) yields s x40.
--   * ON CONFLICT relies on the uniqueIndex GORM created on articles.slug.

INSERT OR IGNORE INTO articles (
    created_at, updated_at, title, slug, content, excerpt, author_id,
    status, post_type, format, visibility, published_at, locale
)
WITH RECURSIVE seq (n) AS (
    SELECT 1
    UNION ALL
    SELECT n + 1 FROM seq WHERE n < 10000
)
SELECT
    datetime('now'),
    datetime('now'),
    'Benchmark Article ' || n,
    'benchmark-article-' || n,
    replace(hex(zeroblob(40)), '00', 'ContentX benchmark content for realistic payload size. '),
    'ContentX benchmark excerpt ' || n,
    1,
    'published',
    'post',
    'standard',
    'public',
    datetime('now'),
    'en'
FROM seq;

ANALYZE;
