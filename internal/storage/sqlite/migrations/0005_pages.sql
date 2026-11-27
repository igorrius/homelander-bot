-- Home page versions: an admin (later an agent) uploads a set of files, previews
-- the draft and publishes it; at most one version is published at a time.

CREATE TABLE page_versions (
    id           INTEGER PRIMARY KEY,
    status       TEXT NOT NULL CHECK (status IN ('draft', 'published', 'archived')),
    note         TEXT NOT NULL DEFAULT '',
    created_by   INTEGER REFERENCES users(id) ON DELETE SET NULL,
    published_by INTEGER REFERENCES users(id) ON DELETE SET NULL,
    created_at   TEXT NOT NULL,
    updated_at   TEXT NOT NULL,
    published_at TEXT
) STRICT;
CREATE UNIQUE INDEX page_versions_one_published ON page_versions(status) WHERE status = 'published';

CREATE TABLE page_files (
    version_id   INTEGER NOT NULL REFERENCES page_versions(id) ON DELETE CASCADE,
    path         TEXT NOT NULL,
    content_type TEXT NOT NULL,
    data         BLOB NOT NULL,
    sha256       TEXT NOT NULL,
    size         INTEGER NOT NULL,
    PRIMARY KEY (version_id, path)
) STRICT;
