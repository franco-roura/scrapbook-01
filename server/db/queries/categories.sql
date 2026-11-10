-- name: ListCategories :many
SELECT c.id, c.name, c.created_at, count(i.id)::int AS item_count
FROM categories c
LEFT JOIN items i ON i.category_id = c.id
WHERE c.user_id = @user_id
GROUP BY c.id
ORDER BY lower(c.name);

-- name: CategoryBelongsToUser :one
SELECT EXISTS (SELECT 1 FROM categories WHERE id = @id AND user_id = @user_id)::bool;

-- name: InsertCategory :one
-- Creating a name that exists (ignoring case) returns the existing category.
INSERT INTO categories (user_id, name, name_embedding)
VALUES (@user_id, @name, CAST(sqlc.narg(name_embedding)::text AS vector))
ON CONFLICT (user_id, lower(name)) DO UPDATE
SET name_embedding = COALESCE(categories.name_embedding, EXCLUDED.name_embedding)
RETURNING id, name;

-- name: InsertCategoryFeedback :exec
INSERT INTO category_feedback (user_id, item_id, from_category_id, to_category_id, description)
VALUES (@user_id, @item_id, sqlc.narg(from_category_id), sqlc.narg(to_category_id), @description);
