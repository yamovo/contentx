-- MySQL benchmark seed: 10,000 published articles.
-- Row-for-row identical to seed_postgres_10000.sql so the cross-database
-- comparison uses the same dataset.
--
-- Usage (from host, with the benchmark stack running):
--   mysql -h127.0.0.1 -P13306 -ucontentx -p<password> contentx < seed_mysql_10000.sql
--
-- Requires MySQL 8.0+ (recursive CTE). The application must have started at
-- least once so GORM AutoMigrate created the `articles` table.

SET SESSION cte_max_recursion_depth = 20000;

INSERT IGNORE INTO articles (
    created_at, updated_at, title, slug, content, excerpt, author_id,
    status, post_type, format, visibility, published_at, locale
)
WITH RECURSIVE seq (n) AS (
    SELECT 1
    UNION ALL
    SELECT n + 1 FROM seq WHERE n < 10000
)
SELECT
    NOW(),
    NOW(),
    CONCAT('Benchmark Article ', n),
    CONCAT('benchmark-article-', n),
    REPEAT('ContentX benchmark content for realistic payload size. ', 40),
    CONCAT('ContentX benchmark excerpt ', n),
    1,
    'published',
    'post',
    'standard',
    'public',
    NOW(),
    'en'
FROM seq;

ANALYZE TABLE articles;
