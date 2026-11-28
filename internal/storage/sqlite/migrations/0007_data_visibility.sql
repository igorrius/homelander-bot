-- Data visibility: the public flag becomes three levels (resident cabinet
-- design §4). public: anyone; residents: signed-in residents and admins;
-- admins: only admins. A private item becomes admins-only, so nothing is
-- more visible than before.
ALTER TABLE data_items ADD COLUMN visibility TEXT NOT NULL DEFAULT 'admins'
    CHECK (visibility IN ('public', 'residents', 'admins'));
UPDATE data_items SET visibility = CASE is_public WHEN 1 THEN 'public' ELSE 'admins' END;
ALTER TABLE data_items DROP COLUMN is_public;
