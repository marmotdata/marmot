ALTER TABLE assets ADD COLUMN version BIGINT NOT NULL DEFAULT 1 CHECK (version > 0);

COMMENT ON COLUMN assets.version IS 'Optimistic concurrency token for asset row updates';

---- create above / drop below ----

ALTER TABLE assets DROP COLUMN version;
