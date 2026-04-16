CREATE TABLE asset_schedules (
    asset_id    VARCHAR(255) NOT NULL REFERENCES assets(id) ON DELETE CASCADE,
    schedule_id UUID         NOT NULL REFERENCES ingestion_schedules(id) ON DELETE CASCADE,
    created_at  TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    PRIMARY KEY (asset_id, schedule_id)
);

CREATE INDEX idx_asset_schedules_asset_id    ON asset_schedules(asset_id);
CREATE INDEX idx_asset_schedules_schedule_id ON asset_schedules(schedule_id);

---- create above / drop below ----

DROP TABLE IF EXISTS asset_schedules;
