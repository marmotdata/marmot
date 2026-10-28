-- Plugins that run outside Marmot (`marmot ingest --serve`) answer queries
-- over a mutual TLS connection they open to Marmot; Marmot never connects out
-- to them. The instance id names this Marmot in every serving certificate and
-- signed query, and never changes.
CREATE TABLE plugin_serve_instance (
    singleton   BOOLEAN PRIMARY KEY DEFAULT true CHECK (singleton),
    instance_id UUID NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Generations of the private CA and the key that signs every query. A key is
-- either stored here, encrypted with the server encryption key, or held in a
-- KMS and named by URI, in which case it never leaves the KMS. A generation is
-- trusted from creation until retires_at and used from activates_at, so a
-- rotation publishes the new keys well before signing with them and keeps the
-- old ones trusted until nothing issued under them is still live.
CREATE TABLE plugin_serve_keys (
    id                 UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    ca_cert            TEXT NOT NULL,
    ca_key             TEXT,
    ca_key_uri         TEXT,
    signing_public_key TEXT NOT NULL,
    signing_key        TEXT,
    signing_key_uri    TEXT,
    created_at         TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    activates_at       TIMESTAMPTZ NOT NULL,
    retires_at         TIMESTAMPTZ,
    CONSTRAINT plugin_serve_keys_ca_key_one CHECK ((ca_key IS NULL) <> (ca_key_uri IS NULL)),
    CONSTRAINT plugin_serve_keys_signing_key_one CHECK ((signing_key IS NULL) <> (signing_key_uri IS NULL))
);

CREATE INDEX idx_plugin_serve_keys_activates ON plugin_serve_keys (activates_at);

-- One row per short-lived certificate issued to a serving process. A process
-- enrols with its ordinary Marmot credential and enrols again before each
-- certificate expires. Every replica re-checks each live connection's
-- principal and credential as it goes, so deactivating the principal,
-- removing its serve permission or deleting its API key cuts the process off
-- within seconds.
CREATE TABLE plugin_serve_enrolments (
    id                UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    principal_type    VARCHAR(20) NOT NULL,
    principal_id      UUID NOT NULL,
    audit_subject     TEXT NOT NULL,
    sources           TEXT[] NOT NULL,
    public_key_sha256 BYTEA NOT NULL,
    -- The credential the process enrolled with. For API keys this is the
    -- key's SHA-256 lookup, so every replica can see within one sweep that
    -- the key was deleted or has expired and close the connection.
    credential_type   VARCHAR(40) NOT NULL,
    credential_lookup TEXT,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    expires_at        TIMESTAMPTZ NOT NULL
);

CREATE INDEX idx_plugin_serve_enrolments_sources ON plugin_serve_enrolments USING GIN (sources);
CREATE INDEX idx_plugin_serve_enrolments_principal ON plugin_serve_enrolments (principal_type, principal_id);

-- The replica holding each live connection, so a query arriving at any
-- replica can be forwarded to the one the plugin is connected to.
CREATE TABLE plugin_serve_sessions (
    enrolment_id    UUID NOT NULL REFERENCES plugin_serve_enrolments(id) ON DELETE CASCADE,
    replica_id      UUID NOT NULL,
    replica_address TEXT NOT NULL,
    heartbeat_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (enrolment_id, replica_id)
);

-- The principal that first serves a source owns its name, so another
-- credential can never take the source over and receive its queries.
ALTER TABLE ingestion_schedules ADD COLUMN served_by_type VARCHAR(20);
ALTER TABLE ingestion_schedules ADD COLUMN served_by_id UUID;
ALTER TABLE ingestion_schedules ADD CONSTRAINT ingestion_schedules_served_by_pair
    CHECK ((served_by_type IS NULL) = (served_by_id IS NULL));

INSERT INTO permissions (name, description, resource_type, action) VALUES
    ('serve_ingestion', 'Serve queries from plugins that run outside Marmot', 'ingestion', 'serve');

INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id FROM roles r, permissions p
WHERE r.name = 'admin' AND p.name = 'serve_ingestion';

---- create above / drop below ----

DELETE FROM role_permissions
 WHERE permission_id IN (SELECT id FROM permissions WHERE name = 'serve_ingestion');
DELETE FROM permissions WHERE name = 'serve_ingestion';

ALTER TABLE ingestion_schedules DROP CONSTRAINT IF EXISTS ingestion_schedules_served_by_pair;
ALTER TABLE ingestion_schedules DROP COLUMN IF EXISTS served_by_id;
ALTER TABLE ingestion_schedules DROP COLUMN IF EXISTS served_by_type;

DROP TABLE IF EXISTS plugin_serve_sessions;
DROP TABLE IF EXISTS plugin_serve_enrolments;
DROP TABLE IF EXISTS plugin_serve_keys;
DROP TABLE IF EXISTS plugin_serve_instance;
