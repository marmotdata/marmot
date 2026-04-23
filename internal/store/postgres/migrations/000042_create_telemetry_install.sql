CREATE TABLE IF NOT EXISTS telemetry_install (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

---- create above / drop below ----

DROP TABLE IF EXISTS telemetry_install;
