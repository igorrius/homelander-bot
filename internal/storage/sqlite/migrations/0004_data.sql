-- Data space: named values and tables admins (later agents and modules) keep
-- in the system; public ones are readable by anyone through the site.

CREATE TABLE data_items (
    id           INTEGER PRIMARY KEY,
    key          TEXT NOT NULL UNIQUE,
    kind         TEXT NOT NULL CHECK (kind IN ('value', 'table')),
    description  TEXT NOT NULL DEFAULT '',
    is_public    INTEGER NOT NULL DEFAULT 0 CHECK (is_public IN (0, 1)),
    value_type   TEXT,
    value_json   TEXT CHECK (value_json IS NULL OR json_valid(value_json)),
    columns_json TEXT CHECK (columns_json IS NULL OR json_valid(columns_json)),
    created_by   INTEGER REFERENCES users(id) ON DELETE SET NULL,
    updated_by   INTEGER REFERENCES users(id) ON DELETE SET NULL,
    created_at   TEXT NOT NULL,
    updated_at   TEXT NOT NULL
) STRICT;

CREATE TABLE data_rows (
    id         INTEGER PRIMARY KEY,
    item_id    INTEGER NOT NULL REFERENCES data_items(id) ON DELETE CASCADE,
    position   INTEGER NOT NULL DEFAULT 0,
    cells_json TEXT NOT NULL CHECK (json_valid(cells_json))
) STRICT;
CREATE INDEX data_rows_item ON data_rows(item_id, position, id);

-- Which assets an item refers to (image values and image cells), so an asset
-- is served publicly only while a public item refers to it.
CREATE TABLE data_asset_refs (
    item_id  INTEGER NOT NULL REFERENCES data_items(id) ON DELETE CASCADE,
    asset_id INTEGER NOT NULL REFERENCES assets(id) ON DELETE CASCADE,
    PRIMARY KEY (item_id, asset_id)
) STRICT;
CREATE INDEX data_asset_refs_asset ON data_asset_refs(asset_id);
