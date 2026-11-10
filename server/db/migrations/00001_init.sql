-- +goose Up
-- user_id holds the Supabase Auth user id (the token's "sub"). There is no
-- foreign key to auth.users so the same schema runs on plain Postgres locally.

CREATE TABLE categories (
    id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id    uuid NOT NULL,
    name       text NOT NULL CHECK (char_length(name) BETWEEN 1 AND 60),
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX categories_user_name_key ON categories (user_id, lower(name));

CREATE TABLE items (
    id            uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id       uuid NOT NULL,
    url           text NOT NULL CHECK (char_length(url) <= 2048),
    platform      text NOT NULL,
    -- What the user said the item is about. NULL means it is still in the
    -- "Undescribed" inbox.
    description   text CHECK (char_length(description) BETWEEN 1 AND 2000),
    category_id   uuid REFERENCES categories (id) ON DELETE SET NULL,
    preview_title text,
    preview_image text,
    created_at    timestamptz NOT NULL DEFAULT now(),
    updated_at    timestamptz NOT NULL DEFAULT now(),
    UNIQUE (user_id, url)
);
CREATE INDEX items_user_recent ON items (user_id, created_at DESC, id DESC);
CREATE INDEX items_user_undescribed ON items (user_id, created_at DESC, id DESC)
    WHERE description IS NULL;

-- +goose Down
DROP TABLE items;
DROP TABLE categories;
