-- Base modules and the information feed (resident cabinet design, part 3).

-- A module owns every key under its code ("feed."); keys an admin made there
-- before move aside under "custom.", so nothing is lost or taken over silently.
UPDATE data_items SET key = 'custom.' || key WHERE key LIKE 'feed.%';

-- Which module writes an item ("module:feed"); NULL for items admins keep.
ALTER TABLE data_items ADD COLUMN owner TEXT;

-- Admin choices per built-in module; a missing row means the module's default.
CREATE TABLE modules (
    code       TEXT PRIMARY KEY,
    enabled    INTEGER NOT NULL CHECK (enabled IN (0, 1)),
    settings   TEXT NOT NULL DEFAULT '{}' CHECK (json_valid(settings)),
    updated_by INTEGER REFERENCES users(id) ON DELETE SET NULL,
    updated_at TEXT NOT NULL
) STRICT;

-- Posts of the information feed. labels: the label ids chosen for Telegram,
-- fixed when the post is published.
CREATE TABLE feed_posts (
    id             INTEGER PRIMARY KEY,
    body           TEXT NOT NULL,
    image_asset_id INTEGER REFERENCES assets(id) ON DELETE SET NULL,
    state          TEXT NOT NULL CHECK (state IN ('draft', 'published')),
    published_at   TEXT,
    labels         TEXT NOT NULL DEFAULT '[]' CHECK (json_valid(labels)),
    created_by     INTEGER REFERENCES users(id) ON DELETE SET NULL,
    updated_by     INTEGER REFERENCES users(id) ON DELETE SET NULL,
    created_at     TEXT NOT NULL,
    updated_at     TEXT NOT NULL,
    deleted_at     TEXT
) STRICT;
CREATE INDEX feed_posts_published ON feed_posts(published_at, id) WHERE state = 'published' AND deleted_at IS NULL;

-- Delivery status of a post is read by the outbox origin ("feed:<id>").
CREATE INDEX outbox_origin ON outbox(origin);
