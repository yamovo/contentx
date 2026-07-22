INSERT INTO articles (
    created_at, updated_at, title, slug, content, excerpt, author_id,
    status, post_type, format, visibility, published_at, locale
)
SELECT
    NOW(),
    NOW(),
    'Benchmark Article ' || n,
    'benchmark-article-' || n,
    repeat('ContentX benchmark content for realistic payload size. ', 40),
    'ContentX benchmark excerpt ' || n,
    1,
    'published',
    'post',
    'standard',
    'public',
    NOW(),
    'en'
FROM generate_series(1, 10000) AS n
ON CONFLICT (slug) DO NOTHING;

ANALYZE articles;
