-- M2: chats and forum topics the bot is in, labels on them, the outbox, and
-- registration links handed out by the bot.

CREATE TABLE chats (
    id                   INTEGER PRIMARY KEY,
    messenger            TEXT NOT NULL,
    external_id          TEXT NOT NULL,
    kind                 TEXT NOT NULL CHECK (kind IN ('group', 'supergroup', 'channel')),
    is_forum             INTEGER NOT NULL DEFAULT 0 CHECK (is_forum IN (0, 1)),
    title                TEXT NOT NULL DEFAULT '',
    state                TEXT NOT NULL CHECK (state IN ('new', 'accepted', 'left')),
    bot_status           TEXT NOT NULL DEFAULT 'unknown' CHECK (bot_status IN ('member', 'administrator', 'restricted', 'unknown')),
    can_send             INTEGER NOT NULL DEFAULT 0 CHECK (can_send IN (0, 1)),
    added_by_external_id TEXT NOT NULL DEFAULT '',
    added_by_user_id     INTEGER REFERENCES users(id) ON DELETE SET NULL,
    accepted_by          INTEGER REFERENCES users(id) ON DELETE SET NULL,
    accepted_at          TEXT,
    leave_at             TEXT,
    left_at              TEXT,
    first_seen_at        TEXT NOT NULL,
    last_seen_at         TEXT NOT NULL,
    UNIQUE (messenger, external_id)
) STRICT;
CREATE INDEX chats_leave_at ON chats(leave_at) WHERE leave_at IS NOT NULL;

CREATE TABLE chat_topics (
    id                 INTEGER PRIMARY KEY,
    chat_id            INTEGER NOT NULL REFERENCES chats(id) ON DELETE CASCADE,
    external_id        TEXT NOT NULL,
    title              TEXT NOT NULL DEFAULT '',
    title_set_by_admin INTEGER NOT NULL DEFAULT 0 CHECK (title_set_by_admin IN (0, 1)),
    gone               INTEGER NOT NULL DEFAULT 0 CHECK (gone IN (0, 1)),
    first_seen_at      TEXT NOT NULL,
    last_seen_at       TEXT NOT NULL,
    UNIQUE (chat_id, external_id)
) STRICT;

CREATE TABLE labels (
    id          INTEGER PRIMARY KEY,
    name        TEXT NOT NULL,
    -- lower-cased name written by the application (SQLite's lower() only knows ASCII)
    name_key    TEXT NOT NULL UNIQUE,
    description TEXT NOT NULL DEFAULT '',
    created_at  TEXT NOT NULL,
    updated_at  TEXT NOT NULL
) STRICT;

-- target_type/target_id point at chats or chat_topics; the application keeps
-- them consistent (one table for every kind of labelled thing).
CREATE TABLE label_bindings (
    label_id    INTEGER NOT NULL REFERENCES labels(id) ON DELETE CASCADE,
    target_type TEXT NOT NULL CHECK (target_type IN ('chat', 'chat_topic')),
    target_id   INTEGER NOT NULL,
    created_at  TEXT NOT NULL,
    PRIMARY KEY (label_id, target_type, target_id)
) STRICT;
CREATE INDEX label_bindings_target ON label_bindings(target_type, target_id);

CREATE TABLE outbox (
    id              INTEGER PRIMARY KEY,
    messenger       TEXT NOT NULL,
    chat_id         TEXT NOT NULL,
    thread_id       TEXT NOT NULL DEFAULT '',
    payload         TEXT NOT NULL CHECK (json_valid(payload)),
    origin          TEXT NOT NULL DEFAULT '',
    label_id        INTEGER REFERENCES labels(id) ON DELETE SET NULL,
    user_id         INTEGER REFERENCES users(id) ON DELETE SET NULL,
    state           TEXT NOT NULL CHECK (state IN ('pending', 'sent', 'failed')),
    attempts        INTEGER NOT NULL DEFAULT 0,
    next_attempt_at TEXT NOT NULL,
    last_error      TEXT NOT NULL DEFAULT '',
    created_at      TEXT NOT NULL,
    sent_at         TEXT
) STRICT;
CREATE INDEX outbox_due ON outbox(state, next_attempt_at);

CREATE TABLE registration_tokens (
    id           INTEGER PRIMARY KEY,
    token_hash   BLOB NOT NULL UNIQUE,
    channel_type TEXT NOT NULL,
    external_id  TEXT NOT NULL,
    chat_id      TEXT NOT NULL,
    username     TEXT NOT NULL DEFAULT '',
    phone        TEXT NOT NULL,
    expires_at   TEXT NOT NULL,
    used_at      TEXT,
    created_at   TEXT NOT NULL
) STRICT;
CREATE INDEX registration_tokens_account ON registration_tokens(channel_type, external_id);
