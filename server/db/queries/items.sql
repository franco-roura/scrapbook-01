-- name: UpsertItem :one
-- Saving the same link again keeps one row. A new description replaces the
-- old one; saving again without a description keeps what was there.
-- described_at only moves when the description text actually changes.
INSERT INTO items (user_id, url, platform, description, described_at)
VALUES (@user_id, @url, @platform, sqlc.narg(description),
        CASE WHEN sqlc.narg(description)::text IS NULL THEN NULL ELSE now() END)
ON CONFLICT (user_id, url) DO UPDATE
SET description  = COALESCE(EXCLUDED.description, items.description),
    described_at = CASE
        WHEN EXCLUDED.description IS NOT NULL AND EXCLUDED.description IS DISTINCT FROM items.description
            THEN now()
        ELSE items.described_at
    END,
    updated_at   = now()
RETURNING id, (xmax = 0)::bool AS inserted,
          (description IS NOT NULL AND classified_at IS DISTINCT FROM described_at)::bool AS needs_classifying;

-- name: GetItem :one
SELECT i.id, i.url, i.platform, i.description, i.preview_title, i.preview_description, i.preview_site, i.preview_image,
       i.created_at, i.updated_at, i.category_id, c.name AS category_name, i.category_source,
       (i.description IS NOT NULL AND i.classified_at IS DISTINCT FROM i.described_at)::bool AS classifying
FROM items i
LEFT JOIN categories c ON c.id = i.category_id
WHERE i.id = @id AND i.user_id = @user_id;

-- name: ListItems :many
-- Newest first, paged with a (created_at, id) cursor.
SELECT i.id, i.url, i.platform, i.description, i.preview_title, i.preview_description, i.preview_site, i.preview_image,
       i.created_at, i.updated_at, i.category_id, c.name AS category_name, i.category_source,
       (i.description IS NOT NULL AND i.classified_at IS DISTINCT FROM i.described_at)::bool AS classifying
FROM items i
LEFT JOIN categories c ON c.id = i.category_id
WHERE i.user_id = @user_id
  AND (NOT @only_undescribed::bool OR i.description IS NULL)
  AND (sqlc.narg(category_id)::uuid IS NULL OR i.category_id = sqlc.narg(category_id)::uuid)
  AND (sqlc.narg(before_created)::timestamptz IS NULL
       OR (i.created_at, i.id) < (sqlc.narg(before_created)::timestamptz, sqlc.narg(before_id)::uuid))
ORDER BY i.created_at DESC, i.id DESC
LIMIT @lim;

-- name: SetItemDescription :one
UPDATE items
SET description  = sqlc.narg(description),
    described_at = CASE
        WHEN sqlc.narg(description)::text IS NOT DISTINCT FROM description THEN described_at
        WHEN sqlc.narg(description)::text IS NULL THEN NULL
        ELSE now()
    END,
    updated_at   = now()
WHERE id = @id AND user_id = @user_id
RETURNING (description IS NOT NULL AND classified_at IS DISTINCT FROM described_at)::bool AS needs_classifying;

-- name: SetItemCategoryByUser :one
-- The user picked a category: the classifier won't change it again.
WITH old AS (
    SELECT category_id, description FROM items
    WHERE id = @id AND user_id = @user_id
    FOR UPDATE
)
UPDATE items i
SET category_id = @category_id, category_source = 'user', updated_at = now()
FROM old
WHERE i.id = @id AND i.user_id = @user_id
RETURNING old.category_id AS from_category_id, old.description;

-- name: DeleteItem :execrows
DELETE FROM items WHERE id = @id AND user_id = @user_id;

-- name: GetItemForPreview :one
SELECT id, url, platform, preview_fetched_at FROM items WHERE id = @id;

-- name: SetItemPreview :exec
-- Records an attempt, found or not. Empty strings are stored as NULL.
UPDATE items
SET preview_title       = NULLIF(@title::text, ''),
    preview_description = NULLIF(@description::text, ''),
    preview_site        = NULLIF(@site::text, ''),
    preview_image       = NULLIF(@image::text, ''),
    preview_fetched_at  = now()
WHERE id = @id;

-- name: ItemsNeedingPreview :many
-- Recent items whose preview was never attempted (a failed invoke, or saved
-- before previews existed). Older ones are left alone.
SELECT id FROM items
WHERE preview_fetched_at IS NULL AND created_at > now() - interval '7 days'
ORDER BY created_at DESC
LIMIT @lim;
