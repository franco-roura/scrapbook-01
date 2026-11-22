-- name: SearchItemsByVector :many
-- Nearest items to a query embedding (cosine), optionally in one category.
-- An item matches through its description or through its analysis, and
-- keeps the better of the two scores. Each side is its own nearest-neighbour
-- scan, so both HNSW indexes stay usable.
WITH hits AS (
    (SELECT i.id, (1 - (i.embedding <=> CAST(sqlc.arg(embedding)::text AS vector)))::float8 AS score
     FROM items i
     LEFT JOIN categories c ON c.id = i.category_id
     WHERE i.user_id = @user_id AND i.embedding IS NOT NULL
       AND (sqlc.narg(category)::text IS NULL OR lower(c.name) = lower(sqlc.narg(category)::text))
     ORDER BY i.embedding <=> CAST(sqlc.arg(embedding)::text AS vector)
     LIMIT @lim)
    UNION ALL
    (SELECT a.item_id, (1 - (a.embedding <=> CAST(sqlc.arg(embedding)::text AS vector)))::float8 AS score
     FROM item_analyses a
     JOIN items i ON i.id = a.item_id
     LEFT JOIN categories c ON c.id = i.category_id
     WHERE a.user_id = @user_id AND a.embedding IS NOT NULL
       AND (sqlc.narg(category)::text IS NULL OR lower(c.name) = lower(sqlc.narg(category)::text))
     ORDER BY a.embedding <=> CAST(sqlc.arg(embedding)::text AS vector)
     LIMIT @lim)
), best AS (
    SELECT hits.id, max(hits.score)::float8 AS score FROM hits GROUP BY hits.id
)
SELECT i.id, i.url, i.platform, i.description, i.created_at, c.name AS category_name,
       a.summary AS analysis_summary, best.score
FROM best
JOIN items i ON i.id = best.id
LEFT JOIN categories c ON c.id = i.category_id
LEFT JOIN item_analyses a ON a.item_id = i.id
ORDER BY best.score DESC, i.created_at DESC
LIMIT @lim;

-- name: SearchItemsByText :many
-- Fallback when embeddings aren't available: items whose description, URL,
-- category or analysis contain the most query words.
SELECT i.id, i.url, i.platform, i.description, i.created_at, c.name AS category_name,
       a.summary AS analysis_summary, m.hits
FROM items i
LEFT JOIN categories c ON c.id = i.category_id
LEFT JOIN item_analyses a ON a.item_id = i.id
CROSS JOIN LATERAL (
    SELECT count(*)::int AS hits FROM unnest(@words::text[]) w
    WHERE concat_ws(' ', i.description, i.url, c.name, a.summary, a.details,
                    array_to_string(a.tags, ' '), array_to_string(a.use_when, ' ')) ILIKE '%' || w || '%'
) m
WHERE i.user_id = @user_id
  AND (sqlc.narg(category)::text IS NULL OR lower(c.name) = lower(sqlc.narg(category)::text))
  AND m.hits > 0
ORDER BY m.hits DESC, i.created_at DESC
LIMIT @lim;
