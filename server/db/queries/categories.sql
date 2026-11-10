-- name: ListCategories :many
SELECT c.id, c.name, c.created_at, count(i.id)::int AS item_count
FROM categories c
LEFT JOIN items i ON i.category_id = c.id
WHERE c.user_id = @user_id
GROUP BY c.id
ORDER BY lower(c.name);
