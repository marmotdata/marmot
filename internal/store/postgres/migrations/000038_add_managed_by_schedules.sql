ALTER TABLE ingestion_schedules ADD COLUMN managed_by VARCHAR(50);

CREATE INDEX idx_ingestion_schedules_managed_by ON ingestion_schedules(managed_by);

---- create above / drop below ----

DROP INDEX IF EXISTS idx_ingestion_schedules_managed_by;
ALTER TABLE ingestion_schedules DROP COLUMN IF EXISTS managed_by;
