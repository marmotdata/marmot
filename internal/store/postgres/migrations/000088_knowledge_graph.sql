-- Knowledge links become typed, and carry the linked entry's kind, state and sort key, so the entries about an asset page from an index in a few milliseconds even when tens of thousands point at it. Measured on 5M links with hot assets of 50,000: the first page fell from 203 ms to 1.6 ms and the next from 775 ms to 1.9 ms.

-- What a status means in the order lists show it: in use first, then not yet in use, then no longer in use. Every kind's statuses are listed in internal/core/knowledge/kinds.go, and a test there checks this function agrees.
CREATE FUNCTION knowledge_state_rank(status TEXT) RETURNS SMALLINT
LANGUAGE sql IMMUTABLE PARALLEL SAFE AS $$
    SELECT CASE
        WHEN status IN ('active', 'accepted') THEN 0
        WHEN status IN ('draft', 'in_review', 'proposed') THEN 1
        WHEN status IN ('superseded', 'retired', 'rejected', 'deprecated') THEN 2
        ELSE 3
    END::smallint
$$;

-- Everything a person would search an entry by: its title, summary, typed fields and write-up pages.
CREATE FUNCTION knowledge_search_text(entry TEXT, fields JSONB) RETURNS tsvector
LANGUAGE sql STABLE AS $$
    SELECT setweight(to_tsvector('simple', COALESCE(a.name, '')), 'A') ||
           setweight(to_tsvector('simple', COALESCE(a.user_description, '') || ' ' || COALESCE(a.description, '')), 'B') ||
           setweight(jsonb_to_tsvector('simple', COALESCE(fields, '{}'::jsonb), '["string"]'), 'B') ||
           setweight(to_tsvector('simple', COALESCE((
               SELECT string_agg(dp.title || ' ' || COALESCE(dp.content, ''), ' ')
               FROM doc_pages dp WHERE dp.entity_type = 'asset' AND dp.entity_id = a.mrn), '')), 'C')
    FROM assets a WHERE a.id = entry
$$;

ALTER TABLE knowledge_entries
    ADD COLUMN state_rank  SMALLINT NOT NULL DEFAULT 3,
    ADD COLUMN search_text tsvector;

CREATE FUNCTION knowledge_entries_derive() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
    NEW.state_rank := knowledge_state_rank(NEW.status);
    NEW.search_text := knowledge_search_text(NEW.asset_id, NEW.fields);
    RETURN NEW;
END;
$$;

CREATE TRIGGER knowledge_entries_derive BEFORE INSERT OR UPDATE ON knowledge_entries
    FOR EACH ROW EXECUTE FUNCTION knowledge_entries_derive();

UPDATE knowledge_entries SET status = status;

CREATE INDEX idx_knowledge_entries_search ON knowledge_entries USING gin (search_text);
CREATE INDEX idx_knowledge_entries_rank   ON knowledge_entries (state_rank, updated_at DESC);

-- Links from an entry to anything in the catalog, by relation. Only the entry's side is written by people; kind, state_rank and name_key are copies kept in step by triggers, so pages of entries about an asset come straight from the index in order.
CREATE TABLE knowledge_links (
    entry_id   VARCHAR(255) NOT NULL REFERENCES assets(id) ON DELETE CASCADE,
    relation   TEXT         NOT NULL,
    target_id  VARCHAR(255) NOT NULL REFERENCES assets(id) ON DELETE CASCADE,
    kind       TEXT         NOT NULL,
    state_rank SMALLINT     NOT NULL DEFAULT 3,
    name_key   TEXT         NOT NULL DEFAULT '',
    created_by TEXT,
    created_at TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    PRIMARY KEY (entry_id, relation, target_id),
    CHECK (entry_id <> target_id)
);

CREATE INDEX idx_knowledge_links_about      ON knowledge_links (target_id, state_rank, name_key, entry_id) INCLUDE (relation, kind);
CREATE INDEX idx_knowledge_links_about_kind ON knowledge_links (target_id, kind, state_rank, name_key, entry_id) INCLUDE (relation);

INSERT INTO knowledge_links (entry_id, relation, target_id, kind, state_rank, name_key, created_at)
SELECT l.entry_id, 'applies_to', l.target_id, k.kind, k.state_rank, lower(COALESCE(a.name, '')), l.created_at
FROM knowledge_applies_to l
JOIN knowledge_entries k ON k.asset_id = l.entry_id
JOIN assets a ON a.id = l.entry_id;

DROP TABLE knowledge_applies_to;

-- Rules link an entry to every asset matching a selector, such as everything tagged pii, without storing a row per asset. They are matched against an asset when it is read, so an asset tagged tomorrow is covered tomorrow. Type and provider values are stored lowercase; tags are matched as written, as tags are elsewhere.
CREATE TABLE knowledge_rules (
    id             UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    entry_id       VARCHAR(255) NOT NULL REFERENCES assets(id) ON DELETE CASCADE,
    relation       TEXT         NOT NULL,
    selector_type  TEXT         NOT NULL CHECK (selector_type IN ('tag', 'type', 'provider')),
    selector_value TEXT         NOT NULL,
    created_by     TEXT,
    created_at     TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    UNIQUE (entry_id, relation, selector_type, selector_value)
);

CREATE INDEX idx_knowledge_rules_selector ON knowledge_rules (selector_type, selector_value);

-- A status change re-ranks the entry's links; a rename or retype re-sorts them; any edit an entry is searched by refreshes its search text.
CREATE FUNCTION knowledge_entries_sync_links() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
    IF NEW.state_rank IS DISTINCT FROM OLD.state_rank OR NEW.kind IS DISTINCT FROM OLD.kind THEN
        UPDATE knowledge_links SET state_rank = NEW.state_rank, kind = NEW.kind WHERE entry_id = NEW.asset_id;
    END IF;
    RETURN NULL;
END;
$$;

CREATE TRIGGER knowledge_entries_sync_links AFTER UPDATE ON knowledge_entries
    FOR EACH ROW EXECUTE FUNCTION knowledge_entries_sync_links();

CREATE FUNCTION knowledge_assets_sync() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
    IF NEW.name IS DISTINCT FROM OLD.name THEN
        UPDATE knowledge_links SET name_key = lower(COALESCE(NEW.name, '')) WHERE entry_id = NEW.id;
    END IF;
    UPDATE knowledge_entries SET updated_at = updated_at WHERE asset_id = NEW.id;
    RETURN NULL;
END;
$$;

CREATE TRIGGER knowledge_assets_sync AFTER UPDATE OF name, description, user_description ON assets
    FOR EACH ROW
    WHEN (OLD.name IS DISTINCT FROM NEW.name OR OLD.description IS DISTINCT FROM NEW.description OR OLD.user_description IS DISTINCT FROM NEW.user_description)
    EXECUTE FUNCTION knowledge_assets_sync();

CREATE FUNCTION knowledge_doc_pages_sync() RETURNS trigger
LANGUAGE plpgsql AS $$
DECLARE
    page doc_pages%ROWTYPE;
BEGIN
    IF TG_OP = 'DELETE' THEN page := OLD; ELSE page := NEW; END IF;
    IF page.entity_type = 'asset' THEN
        UPDATE knowledge_entries k SET updated_at = k.updated_at
        FROM assets a WHERE a.mrn = page.entity_id AND k.asset_id = a.id;
    END IF;
    RETURN NULL;
END;
$$;

CREATE TRIGGER knowledge_doc_pages_sync AFTER INSERT OR UPDATE OF title, content OR DELETE ON doc_pages
    FOR EACH ROW EXECUTE FUNCTION knowledge_doc_pages_sync();

---- create above / drop below ----

DROP TRIGGER IF EXISTS knowledge_doc_pages_sync ON doc_pages;
DROP FUNCTION IF EXISTS knowledge_doc_pages_sync();
DROP TRIGGER IF EXISTS knowledge_assets_sync ON assets;
DROP FUNCTION IF EXISTS knowledge_assets_sync();
DROP TRIGGER IF EXISTS knowledge_entries_sync_links ON knowledge_entries;
DROP FUNCTION IF EXISTS knowledge_entries_sync_links();
DROP TABLE IF EXISTS knowledge_rules;

CREATE TABLE knowledge_applies_to (
    entry_id   VARCHAR(255) NOT NULL REFERENCES assets(id) ON DELETE CASCADE,
    target_id  VARCHAR(255) NOT NULL REFERENCES assets(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    PRIMARY KEY (entry_id, target_id),
    CHECK (entry_id <> target_id)
);
CREATE INDEX idx_knowledge_applies_to_target ON knowledge_applies_to (target_id);
INSERT INTO knowledge_applies_to (entry_id, target_id, created_at)
SELECT DISTINCT ON (entry_id, target_id) entry_id, target_id, created_at FROM knowledge_links;
DROP TABLE IF EXISTS knowledge_links;

DROP TRIGGER IF EXISTS knowledge_entries_derive ON knowledge_entries;
DROP FUNCTION IF EXISTS knowledge_entries_derive();
ALTER TABLE knowledge_entries DROP COLUMN IF EXISTS search_text, DROP COLUMN IF EXISTS state_rank;
DROP FUNCTION IF EXISTS knowledge_search_text(TEXT, JSONB);
DROP FUNCTION IF EXISTS knowledge_state_rank(TEXT);
