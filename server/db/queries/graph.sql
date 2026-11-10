-- name: ListGraphItems :many
-- The newest items for the graph view, with their categories.
SELECT i.id, i.url, i.platform, i.description, i.category_id, c.name AS category_name
FROM items i
LEFT JOIN categories c ON c.id = i.category_id
WHERE i.user_id = @user_id
ORDER BY i.created_at DESC, i.id DESC
LIMIT @lim;
