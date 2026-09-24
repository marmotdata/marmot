CREATE TABLE domain_settings (
    key TEXT PRIMARY KEY,
    value JSONB NOT NULL,
    updated_by TEXT,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- No foreign keys: the log outlives the domains and entities it names.
CREATE TABLE domain_audit_log (
    id BIGSERIAL PRIMARY KEY,
    actor TEXT NOT NULL,
    action TEXT NOT NULL,
    entity_kind TEXT NOT NULL,
    entity_id TEXT NOT NULL,
    from_domain UUID,
    to_domain UUID,
    at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX domain_audit_log_entity_idx ON domain_audit_log (entity_kind, entity_id, at DESC);

---- create above / drop below ----

DROP TABLE domain_audit_log;
DROP TABLE domain_settings;
