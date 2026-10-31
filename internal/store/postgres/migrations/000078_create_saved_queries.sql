-- SQL a person keeps to run again from the Query workspace and Home. Saved
-- queries are personal, so they go with their owner. source_name is not a
-- foreign key, matching data_queries, so renaming or removing a source leaves
-- the statement in place to be pointed elsewhere.
CREATE TABLE IF NOT EXISTS saved_queries (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id     UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name        VARCHAR(255) NOT NULL,
    statement   TEXT NOT NULL,
    source_name VARCHAR(255) NOT NULL DEFAULT '',
    last_run_at TIMESTAMPTZ,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_saved_queries_user ON saved_queries (user_id, updated_at DESC);

---- create above / drop below ----

DROP TABLE IF EXISTS saved_queries;
