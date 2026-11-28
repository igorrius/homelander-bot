-- One-tap login links the bot hands a verified user (resident cabinet design, part 2).
-- Only the HMAC of the token is stored; a link works once, for a few minutes.
CREATE TABLE login_links (
    id         INTEGER PRIMARY KEY,
    token_hash BLOB NOT NULL UNIQUE,
    user_id    INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    next       TEXT NOT NULL, -- cabinet path to open after login, starts with /my
    expires_at TEXT NOT NULL,
    used_at    TEXT,
    created_at TEXT NOT NULL
) STRICT;
CREATE INDEX login_links_expires ON login_links(expires_at);
