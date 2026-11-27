-- M1 identity: people, their messenger channels, houses, apartments, login and sessions.

CREATE TABLE users (
    id           INTEGER PRIMARY KEY,
    phone        TEXT NOT NULL UNIQUE,
    first_name   TEXT NOT NULL DEFAULT '',
    last_name    TEXT NOT NULL DEFAULT '',
    locale       TEXT NOT NULL DEFAULT '',
    is_resident  INTEGER NOT NULL DEFAULT 0 CHECK (is_resident IN (0, 1)),
    is_admin     INTEGER NOT NULL DEFAULT 0 CHECK (is_admin IN (0, 1)),
    status       TEXT NOT NULL CHECK (status IN ('pending_approval', 'awaiting_activation', 'active', 'rejected', 'disabled')),
    source       TEXT NOT NULL CHECK (source IN ('application', 'admin', 'bootstrap')),
    approved_by  INTEGER REFERENCES users(id),
    approved_at  TEXT,
    activated_at TEXT,
    created_at   TEXT NOT NULL,
    updated_at   TEXT NOT NULL,
    -- lower-cased "first last phone-digits", written by the application for
    -- case-insensitive search (SQLite's lower() only knows ASCII)
    search_text  TEXT NOT NULL DEFAULT ''
) STRICT;
CREATE INDEX users_status ON users(status);

CREATE TABLE channels (
    id          INTEGER PRIMARY KEY,
    user_id     INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    type        TEXT NOT NULL,
    external_id TEXT NOT NULL,
    chat_id     TEXT NOT NULL,
    username    TEXT NOT NULL DEFAULT '',
    linked_at   TEXT NOT NULL,
    UNIQUE (type, external_id),
    UNIQUE (user_id, type)
) STRICT;

CREATE TABLE houses (
    id             INTEGER PRIMARY KEY,
    name           TEXT NOT NULL,
    address        TEXT NOT NULL DEFAULT '',
    latitude       REAL,
    longitude      REAL,
    floors         INTEGER,
    apartment_from INTEGER NOT NULL CHECK (apartment_from >= 1),
    apartment_to   INTEGER NOT NULL,
    sort_order     INTEGER NOT NULL DEFAULT 0,
    archived       INTEGER NOT NULL DEFAULT 0 CHECK (archived IN (0, 1)),
    CHECK (apartment_to >= apartment_from)
) STRICT;

CREATE TABLE residency_statuses (
    id         INTEGER PRIMARY KEY,
    code       TEXT NOT NULL UNIQUE,
    names      TEXT NOT NULL CHECK (json_valid(names)),
    sort_order INTEGER NOT NULL DEFAULT 0,
    archived   INTEGER NOT NULL DEFAULT 0 CHECK (archived IN (0, 1))
) STRICT;
INSERT INTO residency_statuses (code, names, sort_order) VALUES
    ('owner',  '{"uk":"Власник","en":"Owner"}', 1),
    ('tenant', '{"uk":"Орендар","en":"Tenant"}', 2);

CREATE TABLE residencies (
    id               INTEGER PRIMARY KEY,
    user_id          INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    house_id         INTEGER NOT NULL REFERENCES houses(id),
    apartment_number INTEGER NOT NULL,
    status_id        INTEGER NOT NULL REFERENCES residency_statuses(id),
    created_at       TEXT NOT NULL,
    UNIQUE (user_id, house_id, apartment_number)
) STRICT;
CREATE INDEX residencies_apartment ON residencies(house_id, apartment_number);

CREATE TABLE login_codes (
    id         INTEGER PRIMARY KEY,
    user_id    INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    code_hash  BLOB NOT NULL,
    expires_at TEXT NOT NULL,
    attempts   INTEGER NOT NULL DEFAULT 0,
    used_at    TEXT,
    ip         TEXT NOT NULL DEFAULT ''
) STRICT;
CREATE INDEX login_codes_user ON login_codes(user_id);

CREATE TABLE sessions (
    id           INTEGER PRIMARY KEY,
    user_id      INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token_hash   BLOB NOT NULL UNIQUE,
    created_at   TEXT NOT NULL,
    last_seen_at TEXT NOT NULL,
    expires_at   TEXT NOT NULL,
    ip           TEXT NOT NULL DEFAULT '',
    user_agent   TEXT NOT NULL DEFAULT ''
) STRICT;
CREATE INDEX sessions_user ON sessions(user_id);

CREATE TABLE audit_log (
    id            INTEGER PRIMARY KEY,
    at            TEXT NOT NULL,
    actor_user_id INTEGER,
    action        TEXT NOT NULL,
    target        TEXT NOT NULL,
    details       TEXT NOT NULL DEFAULT '{}' CHECK (json_valid(details)),
    ip            TEXT NOT NULL DEFAULT ''
) STRICT;
CREATE INDEX audit_log_target ON audit_log(target, at);
CREATE INDEX audit_log_at ON audit_log(at);

CREATE TABLE assets (
    id           INTEGER PRIMARY KEY,
    kind         TEXT NOT NULL,
    content_type TEXT NOT NULL,
    data         BLOB NOT NULL,
    sha256       TEXT NOT NULL,
    uploaded_by  INTEGER NOT NULL,
    uploaded_at  TEXT NOT NULL
) STRICT;
