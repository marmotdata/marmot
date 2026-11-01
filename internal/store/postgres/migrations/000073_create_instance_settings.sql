-- Settings an administrator changes from the UI rather than the config file, such as who may sign in and how. Each key holds one JSON document owned by the feature that reads it, so adding a setting needs no migration.
CREATE TABLE IF NOT EXISTS instance_settings (
    key        TEXT PRIMARY KEY,
    value      JSONB NOT NULL,
    updated_by UUID REFERENCES users(id) ON DELETE SET NULL,
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

---- create above / drop below ----

DROP TABLE IF EXISTS instance_settings;
