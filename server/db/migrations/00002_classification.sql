-- +goose Up
-- Phase 3: automatic categories, embeddings, graph edges and push devices.
-- Embeddings are 1536-dimensional (text-embedding-3-small). Changing the
-- embedding model to one with another size needs a new migration and a
-- re-embed of every item.

CREATE EXTENSION IF NOT EXISTS vector;

ALTER TABLE items
    ADD COLUMN embedding       vector(1536),
    -- 'auto' while the classifier owns the category; 'user' once the user
    -- picked one, after which the classifier leaves it alone.
    ADD COLUMN category_source text NOT NULL DEFAULT 'auto'
        CHECK (category_source IN ('auto', 'user')),
    -- When the description last changed, and which of those versions the
    -- classifier last processed. When they differ, the item needs
    -- (re)classifying. Re-saving a link without a new description doesn't
    -- touch described_at, so it costs no tokens.
    ADD COLUMN described_at    timestamptz,
    ADD COLUMN classified_at   timestamptz;

UPDATE items SET described_at = updated_at WHERE description IS NOT NULL;

CREATE INDEX items_embedding_hnsw ON items USING hnsw (embedding vector_cosine_ops);
CREATE INDEX items_needs_classifying ON items (described_at)
    WHERE description IS NOT NULL AND classified_at IS DISTINCT FROM described_at;

ALTER TABLE categories
    ADD COLUMN name_embedding vector(1536);

-- Corrections the user made to automatic categories. Labeled examples for
-- tuning Jev's threshold later.
CREATE TABLE category_feedback (
    id               uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id          uuid NOT NULL,
    item_id          uuid NOT NULL REFERENCES items (id) ON DELETE CASCADE,
    from_category_id uuid REFERENCES categories (id) ON DELETE SET NULL,
    to_category_id   uuid REFERENCES categories (id) ON DELETE SET NULL,
    description      text NOT NULL,
    created_at       timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX category_feedback_user ON category_feedback (user_id, created_at DESC);

-- Undirected "similar" links between items, stored once with src < dst.
CREATE TABLE edges (
    src     uuid NOT NULL REFERENCES items (id) ON DELETE CASCADE,
    dst     uuid NOT NULL REFERENCES items (id) ON DELETE CASCADE,
    user_id uuid NOT NULL,
    kind    text NOT NULL DEFAULT 'similar' CHECK (kind IN ('similar')),
    weight  real NOT NULL,
    PRIMARY KEY (src, dst, kind),
    CHECK (src < dst)
);
CREATE INDEX edges_dst ON edges (dst);
CREATE INDEX edges_user ON edges (user_id);

-- Expo push tokens, one row per installed app.
CREATE TABLE devices (
    token      text PRIMARY KEY CHECK (char_length(token) <= 512),
    user_id    uuid NOT NULL,
    platform   text NOT NULL CHECK (platform IN ('ios', 'android')),
    updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX devices_user ON devices (user_id);

-- +goose Down
DROP TABLE devices;
DROP TABLE edges;
DROP TABLE category_feedback;
ALTER TABLE categories DROP COLUMN name_embedding;
DROP INDEX items_needs_classifying;
DROP INDEX items_embedding_hnsw;
ALTER TABLE items DROP COLUMN classified_at, DROP COLUMN described_at, DROP COLUMN category_source, DROP COLUMN embedding;
