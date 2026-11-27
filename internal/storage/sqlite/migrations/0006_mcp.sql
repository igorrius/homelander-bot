-- MCP: a user flag that lets an admin connect an AI agent (spec 6.2).
ALTER TABLE users ADD COLUMN is_mcp INTEGER NOT NULL DEFAULT 0 CHECK (is_mcp IN (0,1));

-- OAuth 2.1 authorization server for MCP agents (spec 6.1). Secrets (codes and
-- tokens) are stored only as SHA-256 hex.
CREATE TABLE mcp_clients (
    id            TEXT PRIMARY KEY,
    name          TEXT NOT NULL,
    redirect_uris TEXT NOT NULL, -- JSON array
    created_at    TEXT NOT NULL
);

-- A grant is one connected agent: one user and one client.
CREATE TABLE mcp_grants (
    id           INTEGER PRIMARY KEY,
    user_id      INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    client_id    TEXT NOT NULL REFERENCES mcp_clients(id) ON DELETE CASCADE,
    created_at   TEXT NOT NULL,
    last_used_at TEXT,
    revoked_at   TEXT,
    UNIQUE (user_id, client_id)
);

CREATE TABLE mcp_codes (
    hash         TEXT PRIMARY KEY,
    grant_id     INTEGER NOT NULL REFERENCES mcp_grants(id) ON DELETE CASCADE,
    redirect_uri TEXT NOT NULL,
    challenge    TEXT NOT NULL,
    resource     TEXT NOT NULL DEFAULT '',
    expires_at   TEXT NOT NULL,
    used_at      TEXT
);

CREATE TABLE mcp_tokens (
    hash       TEXT PRIMARY KEY,
    grant_id   INTEGER NOT NULL REFERENCES mcp_grants(id) ON DELETE CASCADE,
    kind       TEXT NOT NULL CHECK (kind IN ('access', 'refresh')),
    expires_at TEXT NOT NULL,
    revoked_at TEXT
);
CREATE INDEX mcp_tokens_grant ON mcp_tokens (grant_id);
