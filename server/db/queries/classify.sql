-- name: GetItemForClassify :one
SELECT id, user_id, description, described_at, classified_at, category_id, category_source
FROM items
WHERE id = @id;

-- name: ListCategoriesForClassify :many
-- Each category with up to three recent item descriptions, which tell Jev
-- what the user actually files there.
SELECT c.id, c.name,
       COALESCE((
           SELECT array_agg(s.description)
           FROM (
               SELECT i.description FROM items i
               WHERE i.category_id = c.id AND i.description IS NOT NULL
               ORDER BY i.created_at DESC
               LIMIT 3
           ) s
       ), '{}')::text[] AS examples
FROM categories c
WHERE c.user_id = @user_id
ORDER BY lower(c.name)
LIMIT 250;

-- name: SetItemEmbedding :exec
UPDATE items SET embedding = CAST(sqlc.arg(embedding)::text AS vector) WHERE id = @id;

-- name: FindSimilarCategory :one
-- The user's category whose name is closest in meaning to a proposed name.
SELECT id, name, (1 - (name_embedding <=> CAST(sqlc.arg(embedding)::text AS vector)))::float8 AS similarity
FROM categories
WHERE user_id = @user_id AND name_embedding IS NOT NULL
ORDER BY name_embedding <=> CAST(sqlc.arg(embedding)::text AS vector)
LIMIT 1;

-- name: AssignAutoCategory :execrows
-- Only while the classifier owns the category and the description hasn't
-- changed since this job read it.
UPDATE items
SET category_id = @category_id
WHERE id = @id AND category_source = 'auto' AND described_at = @described_at;

-- name: MarkClassified :execrows
UPDATE items SET classified_at = described_at
WHERE id = @id AND described_at = @described_at;

-- name: DeleteEdgesForItem :exec
DELETE FROM edges WHERE src = @item_id OR dst = @item_id;

-- name: InsertSimilarEdges :exec
-- Links an item to its nearest neighbours by embedding (cosine), keeping
-- only reasonably similar ones. Each pair is stored once with src < dst.
INSERT INTO edges (src, dst, user_id, kind, weight)
SELECT LEAST(@item_id::uuid, n.id), GREATEST(@item_id::uuid, n.id), @user_id::uuid, 'similar', n.similarity
FROM (
    SELECT i.id, (1 - (i.embedding <=> t.embedding))::real AS similarity
    FROM items i, (SELECT embedding FROM items WHERE id = @item_id::uuid) t
    WHERE i.user_id = @user_id::uuid AND i.id <> @item_id::uuid AND i.embedding IS NOT NULL
    ORDER BY i.embedding <=> t.embedding
    LIMIT sqlc.arg(k)::int
) n
WHERE n.similarity >= sqlc.arg(min_similarity)::real
ON CONFLICT (src, dst, kind) DO UPDATE SET weight = EXCLUDED.weight;

-- name: ItemsNeedingClassification :many
SELECT id FROM items
WHERE description IS NOT NULL AND classified_at IS DISTINCT FROM described_at
ORDER BY described_at
LIMIT @lim;

-- name: ListEdgesForUser :many
SELECT src, dst, weight FROM edges WHERE user_id = @user_id;
