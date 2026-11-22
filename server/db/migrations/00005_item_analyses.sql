-- +goose Up
-- A structured analysis of a saved item, written by the user's own AI
-- assistant over MCP (save_analysis). The server never reads the post: the
-- assistant writes what it learned from content the user showed it or that
-- the assistant opened itself, the same way the user writes a description.
--
-- One analysis per item; saving again replaces it. The embedding covers the
-- summary, tags and "use when" situations, so search finds an item by what
-- it actually contains, not only by the user's short note.
CREATE TABLE item_analyses (
    item_id    uuid PRIMARY KEY REFERENCES items (id) ON DELETE CASCADE,
    user_id    uuid NOT NULL,
    -- The central idea, in two or three sentences.
    summary    text NOT NULL CHECK (char_length(summary) BETWEEN 1 AND 1000),
    -- The concrete content: lists, steps, places, prices, names, links.
    details    text CHECK (char_length(details) <= 8000),
    tags       text[] NOT NULL DEFAULT '{}' CHECK (cardinality(tags) <= 20),
    -- Situations in which the assistant should bring this item up on its
    -- own, e.g. "planning a trip to Buenos Aires".
    use_when   text[] NOT NULL DEFAULT '{}' CHECK (cardinality(use_when) <= 10),
    -- Things the user could do with it, e.g. "claim the business profile".
    actions    text[] NOT NULL DEFAULT '{}' CHECK (cardinality(actions) <= 10),
    -- What to keep in mind: sponsored content, missing audio, outdated info.
    caveats    text CHECK (char_length(caveats) <= 1000),
    -- Where the analysis came from, e.g. {caption, video_frames, transcript}.
    sources    text[] NOT NULL DEFAULT '{}' CHECK (cardinality(sources) <= 10),
    embedding  vector(1536),
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX item_analyses_user ON item_analyses (user_id);
CREATE INDEX item_analyses_embedding_hnsw ON item_analyses USING hnsw (embedding vector_cosine_ops);

-- +goose Down
DROP TABLE item_analyses;
