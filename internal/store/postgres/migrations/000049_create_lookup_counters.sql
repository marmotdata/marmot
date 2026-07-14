CREATE TABLE IF NOT EXISTS lookup_counters (
    install_id     UUID        NOT NULL,
    source         TEXT        NOT NULL,
    category       TEXT        NOT NULL,
    count          BIGINT      NOT NULL DEFAULT 0,
    reported_count BIGINT      NOT NULL DEFAULT 0,
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (install_id, source, category)
);

---- create above / drop below ----

DROP TABLE IF EXISTS lookup_counters;
