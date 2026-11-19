-- +goose Up
-- Link previews: the title, description, site and image a link publishes
-- (Open Graph or oEmbed), for display only. preview_title and preview_image
-- exist since 00001; preview_image now holds our own copy's URL.
-- preview_fetched_at is set after every attempt, found or not, so a link
-- that has no preview isn't fetched again and again.
ALTER TABLE items
    ADD COLUMN preview_description text,
    ADD COLUMN preview_site        text,
    ADD COLUMN preview_fetched_at  timestamptz;

-- The sweep looks for recent items that were never attempted.
CREATE INDEX items_preview_pending ON items (created_at)
    WHERE preview_fetched_at IS NULL;

-- +goose Down
DROP INDEX items_preview_pending;
ALTER TABLE items
    DROP COLUMN preview_fetched_at,
    DROP COLUMN preview_site,
    DROP COLUMN preview_description;
