-- The version a core plugin runs at when an admin or the auto-update policy has moved it off the version this Marmot release pins. A plugin with no row runs the release's pin. Every replica reads this, so they all run the same version.
CREATE TABLE IF NOT EXISTS core_plugin_versions (
    plugin_id        TEXT PRIMARY KEY,
    version          TEXT NOT NULL,
    digest           TEXT NOT NULL,
    source           TEXT NOT NULL CHECK (source IN ('manual', 'auto')),
    -- A pinned plugin is never moved by auto-update or by a Marmot upgrade raising the release's pin.
    pinned           BOOLEAN NOT NULL DEFAULT FALSE,
    previous_version TEXT,
    previous_digest  TEXT,
    -- The version the running Marmot release pinned when this was chosen. A later release pinning something newer than both supersedes the choice unless it is pinned.
    release_version  TEXT,
    updated_by       TEXT,
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- The last plugin index Marmot verified, one row. Replicas read it from here rather than each fetching it, and its version is the floor below which a fetched index is refused as a replay.
CREATE TABLE IF NOT EXISTS core_plugin_index (
    id         BOOLEAN PRIMARY KEY DEFAULT TRUE CHECK (id),
    version    BIGINT,
    body       BYTEA,
    signature  BYTEA,
    expires_at TIMESTAMPTZ,
    fetched_at TIMESTAMPTZ,
    checked_at TIMESTAMPTZ,
    last_error TEXT
);

---- create above / drop below ----

DROP TABLE IF EXISTS core_plugin_index;
DROP TABLE IF EXISTS core_plugin_versions;
