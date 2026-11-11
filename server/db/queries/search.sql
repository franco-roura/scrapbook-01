-- name: SearchItemsByVector :many
-- Nearest items to a query embedding (cosine), optionally in one category.
SELECT i.id, i.url, i.platform, i.description, i.created_at, c.name AS category_name,
       (1 - (i.embedding <=> CAST(sqlc.arg(embedding)::text AS vector)))::float8 AS score
FROM items i
LEFT JOIN categories c ON c.id = i.category_id
WHERE i.user_id = @user_id AND i.embedding IS NOT NULL
  AND (sqlc.narg(category)::text IS NULL OR lower(c.name) = lower(sqlc.narg(category)::text))
ORDER BY i.embedding <=> CAST(sqlc.arg(embedding)::text AS vector)
LIMIT @lim;

-- name: SearchItemsByText :many
-- Fallback when embeddings aren't available: items whose description, URL
-- or category contain the most query words.
SELECT i.id, i.url, i.platform, i.description, i.created_at, c.name AS category_name, m.hits
FROM items i
LEFT JOIN categories c ON c.id = i.category_id
CROSS JOIN LATERAL (
    SELECT count(*)::int AS hits FROM unnest(@words::text[]) w
    WHERE concat_ws(' ', i.description, i.url, c.name) ILIKE '%' || w || '%'
) m
WHERE i.user_id = @user_id
  AND (sqlc.narg(category)::text IS NULL OR lower(c.name) = lower(sqlc.narg(category)::text))
  AND m.hits > 0
ORDER BY m.hits DESC, i.created_at DESC
LIMIT @lim;
