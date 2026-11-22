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

-- name: GetAnalysisStatus :one
-- When the user's library was last analyzed, and how many saves still need
-- an analysis: never analyzed, or saved again (a new note) since.
-- last_analysis_at is only meaningful when analyzed > 0.
SELECT COALESCE((SELECT max(a.updated_at) FROM item_analyses a WHERE a.user_id = @user_id),
                'epoch'::timestamptz)::timestamptz AS last_analysis_at,
       (SELECT count(*) FROM item_analyses a WHERE a.user_id = @user_id)::int AS analyzed,
       (SELECT count(*) FROM items i WHERE i.user_id = @user_id)::int AS total,
       (SELECT count(*) FROM items i
        LEFT JOIN item_analyses a ON a.item_id = i.id
        WHERE i.user_id = @user_id AND (a.item_id IS NULL OR a.updated_at < i.updated_at))::int AS pending;

-- name: ListItemsToAnalyze :many
-- Saves that need an analysis: never-analyzed ones first, newest first, then
-- the ones saved again since their analysis.
SELECT i.id, i.url, i.platform, i.description, i.created_at, c.name AS category_name,
       a.updated_at AS analyzed_at
FROM items i
LEFT JOIN categories c ON c.id = i.category_id
LEFT JOIN item_analyses a ON a.item_id = i.id
WHERE i.user_id = @user_id AND (a.item_id IS NULL OR a.updated_at < i.updated_at)
ORDER BY (a.item_id IS NOT NULL), i.created_at DESC, i.id
LIMIT @lim;
