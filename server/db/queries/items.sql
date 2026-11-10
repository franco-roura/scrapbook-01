-- name: UpsertItem :one
-- Saving the same link again keeps one row. A new description replaces the
-- old one; saving again without a description keeps what was there.
INSERT INTO items (user_id, url, platform, description)
VALUES (@user_id, @url, @platform, sqlc.narg(description))
ON CONFLICT (user_id, url) DO UPDATE
SET description = COALESCE(EXCLUDED.description, items.description),
    updated_at  = now()
RETURNING id, (xmax = 0)::bool AS inserted;

-- name: GetItem :one
SELECT i.id, i.url, i.platform, i.description, i.preview_title, i.preview_image,
       i.created_at, i.updated_at, i.category_id, c.name AS category_name
FROM items i
LEFT JOIN categories c ON c.id = i.category_id
WHERE i.id = @id AND i.user_id = @user_id;

-- name: ListItems :many
-- Newest first, paged with a (created_at, id) cursor.
SELECT i.id, i.url, i.platform, i.description, i.preview_title, i.preview_image,
       i.created_at, i.updated_at, i.category_id, c.name AS category_name
FROM items i
LEFT JOIN categories c ON c.id = i.category_id
WHERE i.user_id = @user_id
  AND (NOT @only_undescribed::bool OR i.description IS NULL)
  AND (sqlc.narg(before_created)::timestamptz IS NULL
       OR (i.created_at, i.id) < (sqlc.narg(before_created)::timestamptz, sqlc.narg(before_id)::uuid))
ORDER BY i.created_at DESC, i.id DESC
LIMIT @lim;

-- name: SetItemDescription :execrows
UPDATE items
SET description = sqlc.narg(description), updated_at = now()
WHERE id = @id AND user_id = @user_id;

-- name: DeleteItem :execrows
DELETE FROM items WHERE id = @id AND user_id = @user_id;
