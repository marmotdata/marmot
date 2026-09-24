-- How much each memory is used, for ranking. use_score counts uses and
-- decays with a 14-day half-life from used_at: a write or an edit counts as
-- a use, and so does each search that returns the memory.

ALTER TABLE memories
    ADD COLUMN IF NOT EXISTS found_count   INTEGER NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS last_found_at TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS use_score     DOUBLE PRECISION NOT NULL DEFAULT 1,
    ADD COLUMN IF NOT EXISTS used_at       TIMESTAMPTZ NOT NULL DEFAULT NOW();

UPDATE memories SET used_at = updated_at;

---- create above / drop below ----

ALTER TABLE memories
    DROP COLUMN IF EXISTS used_at,
    DROP COLUMN IF EXISTS use_score,
    DROP COLUMN IF EXISTS last_found_at,
    DROP COLUMN IF EXISTS found_count;
