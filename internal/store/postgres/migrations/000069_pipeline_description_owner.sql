-- A pipeline says what it catalogs and who looks after it, so a failed run
-- reaches someone who knows the source. The owner is one user or one team;
-- removing either leaves the pipeline unowned rather than deleting it.
ALTER TABLE ingestion_schedules
    ADD COLUMN IF NOT EXISTS description   TEXT,
    ADD COLUMN IF NOT EXISTS owner_user_id UUID REFERENCES users(id) ON DELETE SET NULL,
    ADD COLUMN IF NOT EXISTS owner_team_id UUID REFERENCES teams(id) ON DELETE SET NULL;

ALTER TABLE ingestion_schedules ADD CONSTRAINT ingestion_schedules_one_owner
    CHECK (owner_user_id IS NULL OR owner_team_id IS NULL);

---- create above / drop below ----

ALTER TABLE ingestion_schedules DROP CONSTRAINT IF EXISTS ingestion_schedules_one_owner;
ALTER TABLE ingestion_schedules
    DROP COLUMN IF EXISTS owner_team_id,
    DROP COLUMN IF EXISTS owner_user_id,
    DROP COLUMN IF EXISTS description;
