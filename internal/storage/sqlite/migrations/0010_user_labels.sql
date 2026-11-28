-- Labels on users (docs/superpowers/specs/2026-11-28-user-labels-design.md):
-- a label can also hang on a user, who then gets what is sent to it in a
-- private bot message. SQLite cannot change a CHECK in place, so the table is
-- rebuilt with its rows; nothing references label_bindings.
CREATE TABLE label_bindings_new (
    label_id    INTEGER NOT NULL REFERENCES labels(id) ON DELETE CASCADE,
    target_type TEXT NOT NULL CHECK (target_type IN ('chat', 'chat_topic', 'user')),
    target_id   INTEGER NOT NULL,
    created_at  TEXT NOT NULL,
    PRIMARY KEY (label_id, target_type, target_id)
) STRICT;
INSERT INTO label_bindings_new (label_id, target_type, target_id, created_at)
    SELECT label_id, target_type, target_id, created_at FROM label_bindings;
DROP TABLE label_bindings;
ALTER TABLE label_bindings_new RENAME TO label_bindings;
CREATE INDEX label_bindings_target ON label_bindings(target_type, target_id);
