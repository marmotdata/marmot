-- A knowledge entry is an asset of a knowledge type. Its typed fields live here, beside the asset, so plugin runs that rewrite an asset's metadata never touch what a person wrote.
CREATE TABLE knowledge_entries (
    asset_id         VARCHAR(255) PRIMARY KEY REFERENCES assets(id) ON DELETE CASCADE,
    kind             TEXT         NOT NULL,
    status           TEXT         NOT NULL,
    fields           JSONB        NOT NULL DEFAULT '{}'::jsonb,
    effective_from   DATE,
    review_by        DATE,
    last_reviewed_at TIMESTAMPTZ,
    last_reviewed_by TEXT,
    -- Only the newer entry stores the link, so an old entry learns it was replaced without anyone editing it.
    supersedes       VARCHAR(255) REFERENCES assets(id) ON DELETE SET NULL,
    updated_by       TEXT,
    created_at       TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at       TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    CHECK (supersedes IS NULL OR supersedes <> asset_id)
);

CREATE INDEX idx_knowledge_entries_kind_status ON knowledge_entries (kind, status);
CREATE INDEX idx_knowledge_entries_review_by   ON knowledge_entries (review_by) WHERE review_by IS NOT NULL;
CREATE INDEX idx_knowledge_entries_supersedes  ON knowledge_entries (supersedes) WHERE supersedes IS NOT NULL;

-- Which assets an entry applies to, such as a retention policy and the tables it governs. Kept apart from lineage, which is data flow.
CREATE TABLE knowledge_applies_to (
    entry_id   VARCHAR(255) NOT NULL REFERENCES assets(id) ON DELETE CASCADE,
    target_id  VARCHAR(255) NOT NULL REFERENCES assets(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    PRIMARY KEY (entry_id, target_id),
    CHECK (entry_id <> target_id)
);

CREATE INDEX idx_knowledge_applies_to_target ON knowledge_applies_to (target_id);

---- create above / drop below ----

DROP TABLE IF EXISTS knowledge_applies_to;
DROP TABLE IF EXISTS knowledge_entries;
