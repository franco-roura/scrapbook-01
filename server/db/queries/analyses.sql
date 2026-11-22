-- name: UpsertItemAnalysis :one
-- Only for an item the user owns: the SELECT finds nothing otherwise, and
-- nothing is written. Saving again replaces the whole analysis.
INSERT INTO item_analyses (item_id, user_id, summary, details, tags, use_when, actions, caveats, sources, embedding)
SELECT i.id, i.user_id, @summary, sqlc.narg(details), @tags::text[], @use_when::text[], @actions::text[],
       sqlc.narg(caveats), @sources::text[], CAST(sqlc.narg(embedding)::text AS vector)
FROM items i
WHERE i.id = @item_id AND i.user_id = @user_id
ON CONFLICT (item_id) DO UPDATE
SET summary    = EXCLUDED.summary,
    details    = EXCLUDED.details,
    tags       = EXCLUDED.tags,
    use_when   = EXCLUDED.use_when,
    actions    = EXCLUDED.actions,
    caveats    = EXCLUDED.caveats,
    sources    = EXCLUDED.sources,
    embedding  = EXCLUDED.embedding,
    updated_at = now()
RETURNING item_id, (xmax = 0)::bool AS inserted;

-- name: GetItemAnalysis :one
SELECT item_id, summary, details, tags, use_when, actions, caveats, sources, updated_at
FROM item_analyses
WHERE item_id = @item_id AND user_id = @user_id;
