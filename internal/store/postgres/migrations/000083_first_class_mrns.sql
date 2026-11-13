-- MRNs become first-class, kinded identifiers: mrn://asset/<type>/<provider>[@<zone>]/<name> for assets, mrn://glossary-term/<source>/<path> for terms and mrn://data-product/<slug> for products.
-- Every asset MRN stored anywhere is rewritten from mrn://<type>/<provider>/<name> to mrn://asset/<type>/<provider>/<name>, which is one-to-one, so no two existing assets can end up with the same MRN.

-- Helpers mirroring the plugin SDK's mrn package so stored MRNs match what Go builds.

CREATE OR REPLACE FUNCTION marmot_nfc(t text) RETURNS text LANGUAGE plpgsql IMMUTABLE AS $$
BEGIN
    RETURN normalize(t, NFC);
EXCEPTION WHEN OTHERS THEN
    RETURN t;
END
$$;

CREATE OR REPLACE FUNCTION marmot_mrn_sanitize(t text) RETURNS text LANGUAGE sql IMMUTABLE AS $$
    SELECT lower(regexp_replace(marmot_nfc(t), '[/[:space:]]', '-', 'g'))
$$;

CREATE OR REPLACE FUNCTION marmot_mrn_sanitize_segment(t text) RETURNS text LANGUAGE sql IMMUTABLE AS $$
    SELECT replace(marmot_mrn_sanitize(t), '@', '-')
$$;

CREATE OR REPLACE FUNCTION marmot_mrn_slug(t text) RETURNS text LANGUAGE sql IMMUTABLE AS $$
    SELECT trim(BOTH '-' FROM regexp_replace(lower(marmot_nfc(trim(t))), '[^[:alnum:]._]+', '-', 'g'))
$$;

-- marmot_mrn_join_path joins name levels with dots, quoting a level that holds a dot or a quote when there is more than one level.
CREATE OR REPLACE FUNCTION marmot_mrn_join_path(parts text[]) RETURNS text LANGUAGE sql IMMUTABLE AS $$
    SELECT CASE
        WHEN array_length(parts, 1) = 1 THEN parts[1]
        ELSE (
            SELECT string_agg(
                CASE WHEN p ~ '[."]' THEN '"' || replace(p, '"', '""') || '"' ELSE p END,
                '.' ORDER BY i)
            FROM unnest(parts) WITH ORDINALITY AS u(p, i)
        )
    END
$$;

-- marmot_canonical_asset_mrn turns an asset MRN of either form into the kinded form. A string that is neither form comes back unchanged and is then rejected by the assets check constraint.
CREATE OR REPLACE FUNCTION marmot_canonical_asset_mrn(m text) RETURNS text LANGUAGE plpgsql IMMUTABLE AS $$
DECLARE
    rest text;
    parts text[];
    provider text;
    zone_name text := '';
    at int;
BEGIN
    IF m IS NULL OR lower(left(m, 6)) <> 'mrn://' THEN
        RETURN m;
    END IF;
    rest := substr(m, 7);
    parts := string_to_array(rest, '/');
    IF array_length(parts, 1) = 4 AND lower(parts[1]) = 'asset' THEN
        parts := parts[2:4];
    ELSIF array_length(parts, 1) <> 3 THEN
        RETURN m;
    END IF;
    IF parts[1] = '' OR parts[2] = '' OR parts[3] = '' THEN
        RETURN m;
    END IF;
    provider := parts[2];
    at := strpos(provider, '@');
    IF at > 0 THEN
        zone_name := substr(provider, at + 1);
        provider := left(provider, at - 1);
    END IF;
    provider := marmot_mrn_sanitize_segment(provider);
    IF lower(left(zone_name, 5)) = 'host=' THEN
        zone_name := 'host=' || marmot_mrn_sanitize_segment(substr(zone_name, 6));
    ELSE
        zone_name := marmot_mrn_sanitize_segment(zone_name);
    END IF;
    IF zone_name <> '' THEN
        provider := provider || '@' || zone_name;
    END IF;
    RETURN 'mrn://asset/' || marmot_mrn_sanitize(parts[1]) || '/' || provider || '/' || marmot_mrn_sanitize(parts[3]);
END
$$;

-- Wider columns: a fully qualified name with a zone outgrows 255 characters. search_text is generated from name and mrn, which blocks changing their type, so it is dropped and rebuilt exactly as before.

DROP INDEX IF EXISTS idx_assets_search;
DROP INDEX IF EXISTS idx_assets_search_all;
ALTER TABLE assets DROP COLUMN search_text;

ALTER TABLE lineage_edges DROP CONSTRAINT IF EXISTS lineage_edges_source_mrn_fkey;
ALTER TABLE lineage_edges DROP CONSTRAINT IF EXISTS lineage_edges_target_mrn_fkey;

ALTER TABLE assets ALTER COLUMN mrn TYPE TEXT, ALTER COLUMN name TYPE TEXT;
ALTER TABLE lineage_edges ALTER COLUMN source_mrn TYPE TEXT, ALTER COLUMN target_mrn TYPE TEXT, ALTER COLUMN job_mrn TYPE TEXT;
ALTER TABLE run_checkpoints ALTER COLUMN entity_mrn TYPE TEXT;
ALTER TABLE documentation ALTER COLUMN mrn TYPE TEXT;
ALTER TABLE doc_pages ALTER COLUMN entity_id TYPE TEXT;
ALTER TABLE agent_tool_calls ALTER COLUMN target_mrn TYPE TEXT;

ALTER TABLE assets ADD COLUMN search_text tsvector GENERATED ALWAYS AS (
    setweight(to_tsvector('english', COALESCE(name, '')), 'A') ||
    setweight(to_tsvector('english', COALESCE(mrn, '')), 'A') ||
    setweight(to_tsvector('english', COALESCE(type, '')), 'B') ||
    setweight(to_tsvector('english', COALESCE(array_to_text(providers), '')), 'B') ||
    setweight(to_tsvector('english', COALESCE(description, '')), 'C')
) STORED;
CREATE INDEX idx_assets_search ON assets USING gin (search_text) WHERE is_stub = FALSE;
CREATE INDEX idx_assets_search_all ON assets USING gin (search_text);

-- Lineage follows an asset whose MRN changes, which is how an asset moves to a qualified MRN or a zone without losing its edges.
ALTER TABLE lineage_edges ADD CONSTRAINT lineage_edges_source_mrn_fkey
    FOREIGN KEY (source_mrn) REFERENCES assets(mrn) ON UPDATE CASCADE;
ALTER TABLE lineage_edges ADD CONSTRAINT lineage_edges_target_mrn_fkey
    FOREIGN KEY (target_mrn) REFERENCES assets(mrn) ON UPDATE CASCADE;

-- The search index takes the asset's URL from the kinded MRN, which carries the zone.
CREATE OR REPLACE FUNCTION public.sync_asset_search_on_insert()
 RETURNS trigger
 LANGUAGE plpgsql
AS $function$
BEGIN
    INSERT INTO search_index (
        type, entity_id, name, description, search_text, updated_at,
        asset_type, primary_provider, providers, tags, url_path, mrn,
        created_by, created_at, metadata
    )
    SELECT
        'asset', id, name, COALESCE(user_description, description),
        search_text, updated_at,
        type, providers[1], providers, tags,
        '/discover/' || COALESCE(SUBSTRING(mrn FROM '^mrn://asset/(.+)$'), LOWER(type) || '/unknown/' || id),
        mrn,
        created_by, created_at, metadata
    FROM inserted
    WHERE is_stub = FALSE
    ON CONFLICT (type, entity_id) DO UPDATE SET
        name = EXCLUDED.name, description = EXCLUDED.description,
        search_text = EXCLUDED.search_text, updated_at = EXCLUDED.updated_at,
        asset_type = EXCLUDED.asset_type, primary_provider = EXCLUDED.primary_provider,
        providers = EXCLUDED.providers, tags = EXCLUDED.tags, url_path = EXCLUDED.url_path,
        mrn = EXCLUDED.mrn,
        created_by = EXCLUDED.created_by, created_at = EXCLUDED.created_at,
        metadata = EXCLUDED.metadata;

    INSERT INTO asset_tags (asset_id, tag)
    SELECT id, unnest(tags)
    FROM inserted
    WHERE is_stub = FALSE AND tags IS NOT NULL AND array_length(tags, 1) > 0
    ON CONFLICT DO NOTHING;

    -- entity_type count
    INSERT INTO summary_counts (dimension, key, count)
    SELECT 'entity_type', 'asset', COUNT(*)
    FROM inserted
    WHERE is_stub = FALSE
    ON CONFLICT (dimension, key) DO UPDATE SET count = summary_counts.count + EXCLUDED.count;

    -- type dimension
    INSERT INTO summary_counts (dimension, key, count)
    SELECT 'type', type, COUNT(*)
    FROM inserted
    WHERE is_stub = FALSE AND type IS NOT NULL
    GROUP BY type
    ON CONFLICT (dimension, key) DO UPDATE SET count = summary_counts.count + EXCLUDED.count;

    -- provider dimension
    INSERT INTO summary_counts (dimension, key, count)
    SELECT 'provider', unnest(providers), COUNT(*)
    FROM inserted
    WHERE is_stub = FALSE AND providers IS NOT NULL AND array_length(providers, 1) > 0
    GROUP BY unnest(providers)
    ON CONFLICT (dimension, key) DO UPDATE SET count = summary_counts.count + EXCLUDED.count;

    -- tag dimension
    INSERT INTO summary_counts (dimension, key, count)
    SELECT 'tag', unnest(tags), COUNT(*)
    FROM inserted
    WHERE is_stub = FALSE AND tags IS NOT NULL AND array_length(tags, 1) > 0
    GROUP BY unnest(tags)
    ON CONFLICT (dimension, key) DO UPDATE SET count = summary_counts.count + EXCLUDED.count;

    RETURN NULL;
END;
$function$;

CREATE OR REPLACE FUNCTION public.sync_asset_search_on_update()
 RETURNS trigger
 LANGUAGE plpgsql
AS $function$
BEGIN
    -- Decrement old counts
    INSERT INTO summary_counts (dimension, key, count)
    SELECT 'entity_type', 'asset', -COUNT(*)
    FROM deleted
    WHERE is_stub = FALSE
    ON CONFLICT (dimension, key) DO UPDATE SET count = summary_counts.count + EXCLUDED.count;

    INSERT INTO summary_counts (dimension, key, count)
    SELECT 'type', type, -COUNT(*)
    FROM deleted
    WHERE is_stub = FALSE AND type IS NOT NULL
    GROUP BY type
    ON CONFLICT (dimension, key) DO UPDATE SET count = summary_counts.count + EXCLUDED.count;

    INSERT INTO summary_counts (dimension, key, count)
    SELECT 'provider', unnest(providers), -COUNT(*)
    FROM deleted
    WHERE is_stub = FALSE AND providers IS NOT NULL AND array_length(providers, 1) > 0
    GROUP BY unnest(providers)
    ON CONFLICT (dimension, key) DO UPDATE SET count = summary_counts.count + EXCLUDED.count;

    INSERT INTO summary_counts (dimension, key, count)
    SELECT 'tag', unnest(tags), -COUNT(*)
    FROM deleted
    WHERE is_stub = FALSE AND tags IS NOT NULL AND array_length(tags, 1) > 0
    GROUP BY unnest(tags)
    ON CONFLICT (dimension, key) DO UPDATE SET count = summary_counts.count + EXCLUDED.count;

    DELETE FROM asset_tags WHERE asset_id IN (SELECT id FROM deleted);

    -- Insert new data
    INSERT INTO search_index (
        type, entity_id, name, description, search_text, updated_at,
        asset_type, primary_provider, providers, tags, url_path, mrn,
        created_by, created_at, metadata
    )
    SELECT
        'asset', id, name, COALESCE(user_description, description),
        search_text, updated_at,
        type, providers[1], providers, tags,
        '/discover/' || COALESCE(SUBSTRING(mrn FROM '^mrn://asset/(.+)$'), LOWER(type) || '/unknown/' || id),
        mrn,
        created_by, created_at, metadata
    FROM inserted
    WHERE is_stub = FALSE
    ON CONFLICT (type, entity_id) DO UPDATE SET
        name = EXCLUDED.name, description = EXCLUDED.description,
        search_text = EXCLUDED.search_text, updated_at = EXCLUDED.updated_at,
        asset_type = EXCLUDED.asset_type, primary_provider = EXCLUDED.primary_provider,
        providers = EXCLUDED.providers, tags = EXCLUDED.tags, url_path = EXCLUDED.url_path,
        mrn = EXCLUDED.mrn,
        created_by = EXCLUDED.created_by, created_at = EXCLUDED.created_at,
        metadata = EXCLUDED.metadata;

    INSERT INTO asset_tags (asset_id, tag)
    SELECT id, unnest(tags)
    FROM inserted
    WHERE is_stub = FALSE AND tags IS NOT NULL AND array_length(tags, 1) > 0
    ON CONFLICT DO NOTHING;

    -- Increment new counts
    INSERT INTO summary_counts (dimension, key, count)
    SELECT 'entity_type', 'asset', COUNT(*)
    FROM inserted
    WHERE is_stub = FALSE
    ON CONFLICT (dimension, key) DO UPDATE SET count = summary_counts.count + EXCLUDED.count;

    INSERT INTO summary_counts (dimension, key, count)
    SELECT 'type', type, COUNT(*)
    FROM inserted
    WHERE is_stub = FALSE AND type IS NOT NULL
    GROUP BY type
    ON CONFLICT (dimension, key) DO UPDATE SET count = summary_counts.count + EXCLUDED.count;

    INSERT INTO summary_counts (dimension, key, count)
    SELECT 'provider', unnest(providers), COUNT(*)
    FROM inserted
    WHERE is_stub = FALSE AND providers IS NOT NULL AND array_length(providers, 1) > 0
    GROUP BY unnest(providers)
    ON CONFLICT (dimension, key) DO UPDATE SET count = summary_counts.count + EXCLUDED.count;

    INSERT INTO summary_counts (dimension, key, count)
    SELECT 'tag', unnest(tags), COUNT(*)
    FROM inserted
    WHERE is_stub = FALSE AND tags IS NOT NULL AND array_length(tags, 1) > 0
    GROUP BY unnest(tags)
    ON CONFLICT (dimension, key) DO UPDATE SET count = summary_counts.count + EXCLUDED.count;

    DELETE FROM search_index
    WHERE type = 'asset' AND entity_id IN (SELECT id FROM inserted WHERE is_stub = TRUE);

    RETURN NULL;
END;
$function$;

-- Rewrite every asset MRN. Two stored MRNs differing only in case or spacing would canonicalise to the same string; the older keeps it and the newer gets its id appended, so nothing is merged behind anyone's back.

CREATE TEMP TABLE mrn_moves ON COMMIT DROP AS
SELECT id, mrn AS old_mrn, marmot_canonical_asset_mrn(mrn) AS new_mrn,
       row_number() OVER (PARTITION BY marmot_canonical_asset_mrn(mrn) ORDER BY is_stub, created_at, id) AS rank
FROM assets;

UPDATE mrn_moves SET new_mrn = new_mrn || '~' || left(replace(id, '-', ''), 8) WHERE rank > 1;

DO $$
DECLARE
    n int;
BEGIN
    SELECT count(*) INTO n FROM mrn_moves WHERE rank > 1;
    IF n > 0 THEN
        RAISE NOTICE 'first_class_mrns: % assets shared an MRN with another asset once case and spacing were normalised; they were given distinct MRNs ending in ~<id>', n;
    END IF;
END
$$;

UPDATE assets a SET mrn = m.new_mrn FROM mrn_moves m WHERE a.id = m.id AND a.mrn IS DISTINCT FROM m.new_mrn;

UPDATE lineage_edges SET job_mrn = marmot_canonical_asset_mrn(job_mrn) WHERE job_mrn IS NOT NULL AND job_mrn IS DISTINCT FROM marmot_canonical_asset_mrn(job_mrn);

UPDATE run_checkpoints c SET entity_mrn = marmot_canonical_asset_mrn(c.entity_mrn)
WHERE c.entity_type = 'asset' AND c.entity_mrn IS DISTINCT FROM marmot_canonical_asset_mrn(c.entity_mrn)
  AND NOT EXISTS (SELECT 1 FROM run_checkpoints o WHERE o.run_id = c.run_id AND o.entity_type = 'asset' AND o.entity_mrn = marmot_canonical_asset_mrn(c.entity_mrn));

UPDATE run_entities e SET entity_mrn = marmot_canonical_asset_mrn(e.entity_mrn)
WHERE e.entity_type = 'asset' AND e.entity_mrn IS DISTINCT FROM marmot_canonical_asset_mrn(e.entity_mrn)
  AND NOT EXISTS (SELECT 1 FROM run_entities o WHERE o.run_id = e.run_id AND o.entity_type = 'asset' AND o.entity_mrn = marmot_canonical_asset_mrn(e.entity_mrn));

UPDATE documentation d SET mrn = marmot_canonical_asset_mrn(d.mrn)
WHERE d.mrn IS DISTINCT FROM marmot_canonical_asset_mrn(d.mrn)
  AND NOT EXISTS (SELECT 1 FROM documentation o WHERE o.source = d.source AND o.mrn = marmot_canonical_asset_mrn(d.mrn));

UPDATE doc_pages SET entity_id = marmot_canonical_asset_mrn(entity_id)
WHERE entity_type = 'asset' AND entity_id IS DISTINCT FROM marmot_canonical_asset_mrn(entity_id);

UPDATE agent_tool_calls SET target_mrn = marmot_canonical_asset_mrn(target_mrn)
WHERE target_mrn LIKE 'mrn://%' AND target_mrn NOT LIKE 'mrn://source/%' AND target_mrn IS DISTINCT FROM marmot_canonical_asset_mrn(target_mrn);

UPDATE data_queries SET referenced_mrns = (
    SELECT array_agg(CASE WHEN r LIKE 'mrn://source/%' THEN r ELSE marmot_canonical_asset_mrn(r) END ORDER BY i)
    FROM unnest(referenced_mrns) WITH ORDINALITY AS u(r, i))
WHERE referenced_mrns IS NOT NULL AND array_length(referenced_mrns, 1) > 0;

INSERT INTO data_query_table_stats_hourly AS t (bucket, mrn, principal_type, principal_id, audit_subject, decision, queries)
SELECT bucket, marmot_canonical_asset_mrn(mrn), principal_type, principal_id, audit_subject, decision, queries
FROM data_query_table_stats_hourly
WHERE mrn NOT LIKE 'mrn://source/%' AND mrn IS DISTINCT FROM marmot_canonical_asset_mrn(mrn)
ON CONFLICT (bucket, mrn, principal_type, principal_id, decision) DO UPDATE SET queries = t.queries + EXCLUDED.queries;
DELETE FROM data_query_table_stats_hourly WHERE mrn NOT LIKE 'mrn://source/%' AND mrn IS DISTINCT FROM marmot_canonical_asset_mrn(mrn);

-- From here on only kinded asset MRNs can be stored. A writer still on an older release sends the legacy form, which these triggers rewrite instead of rejecting.
ALTER TABLE assets ADD CONSTRAINT assets_mrn_kinded CHECK (mrn LIKE 'mrn://asset/%');

CREATE OR REPLACE FUNCTION marmot_canonicalise_asset_mrn_column() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    IF TG_TABLE_NAME = 'assets' THEN
        NEW.mrn := marmot_canonical_asset_mrn(NEW.mrn);
    ELSIF TG_TABLE_NAME = 'lineage_edges' THEN
        NEW.source_mrn := marmot_canonical_asset_mrn(NEW.source_mrn);
        NEW.target_mrn := marmot_canonical_asset_mrn(NEW.target_mrn);
        NEW.job_mrn := marmot_canonical_asset_mrn(NEW.job_mrn);
    ELSIF TG_TABLE_NAME = 'documentation' THEN
        NEW.mrn := marmot_canonical_asset_mrn(NEW.mrn);
    ELSIF TG_TABLE_NAME = 'doc_pages' AND NEW.entity_type = 'asset' THEN
        NEW.entity_id := marmot_canonical_asset_mrn(NEW.entity_id);
    ELSIF TG_TABLE_NAME IN ('run_checkpoints', 'run_entities') AND NEW.entity_type = 'asset' THEN
        NEW.entity_mrn := marmot_canonical_asset_mrn(NEW.entity_mrn);
    END IF;
    RETURN NEW;
END
$$;

CREATE TRIGGER assets_canonical_mrn BEFORE INSERT OR UPDATE OF mrn ON assets
    FOR EACH ROW EXECUTE FUNCTION marmot_canonicalise_asset_mrn_column();
CREATE TRIGGER lineage_edges_canonical_mrn BEFORE INSERT OR UPDATE OF source_mrn, target_mrn, job_mrn ON lineage_edges
    FOR EACH ROW EXECUTE FUNCTION marmot_canonicalise_asset_mrn_column();
CREATE TRIGGER documentation_canonical_mrn BEFORE INSERT OR UPDATE OF mrn ON documentation
    FOR EACH ROW EXECUTE FUNCTION marmot_canonicalise_asset_mrn_column();
CREATE TRIGGER doc_pages_canonical_mrn BEFORE INSERT OR UPDATE OF entity_id ON doc_pages
    FOR EACH ROW EXECUTE FUNCTION marmot_canonicalise_asset_mrn_column();
CREATE TRIGGER run_checkpoints_canonical_mrn BEFORE INSERT OR UPDATE OF entity_mrn ON run_checkpoints
    FOR EACH ROW EXECUTE FUNCTION marmot_canonicalise_asset_mrn_column();
CREATE TRIGGER run_entities_canonical_mrn BEFORE INSERT OR UPDATE OF entity_mrn ON run_entities
    FOR EACH ROW EXECUTE FUNCTION marmot_canonicalise_asset_mrn_column();

-- Aliases: MRNs a thing used to have. An asset that moves to a qualified MRN or gains a zone, a renamed glossary term or data product, or the loser of a merge keeps answering to its old MRN.
CREATE TABLE mrn_aliases (
    alias TEXT PRIMARY KEY,
    kind VARCHAR(32) NOT NULL,
    entity_id TEXT NOT NULL,
    reason VARCHAR(64) NOT NULL DEFAULT 'moved',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_mrn_aliases_entity ON mrn_aliases (kind, entity_id);

CREATE OR REPLACE FUNCTION marmot_drop_asset_aliases() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    DELETE FROM mrn_aliases WHERE kind = 'asset' AND entity_id = OLD.id;
    RETURN OLD;
END
$$;
CREATE TRIGGER assets_drop_aliases AFTER DELETE ON assets FOR EACH ROW EXECUTE FUNCTION marmot_drop_asset_aliases();

-- Contributions: what each writer last said about an asset. The asset row is the merge of them, so a pipeline, an import and a Terraform resource describing the same asset no longer overwrite each other, and a pipeline that stops seeing an asset only withdraws its own contribution.
CREATE TABLE asset_contributions (
    asset_id VARCHAR(255) NOT NULL REFERENCES assets(id) ON DELETE CASCADE,
    writer TEXT NOT NULL,
    role VARCHAR(16) NOT NULL,
    fields JSONB NOT NULL DEFAULT '{}'::jsonb,
    first_seen_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    last_seen_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (asset_id, writer),
    CHECK (role IN ('manual', 'native', 'federated', 'reference'))
);
CREATE INDEX idx_asset_contributions_writer ON asset_contributions (writer);

-- A run belongs to whoever started it as well as to its pipeline, so ten engineers running the same pipeline file against their own laptops no longer read each other's checkpoints and delete each other's assets as stale.
ALTER TABLE runs ADD COLUMN owner TEXT NOT NULL DEFAULT '';
CREATE INDEX idx_runs_pipeline_source_owner ON runs (pipeline_name, source_name, owner, completed_at DESC);

-- Backfill contributions from what each pipeline last saw. Runs before this release have no owner, so their contribution is under the pipeline and source alone; the first run of each owner takes it over.
INSERT INTO asset_contributions (asset_id, writer, role, fields)
SELECT DISTINCT ON (a.id, w.writer)
    a.id, w.writer,
    CASE WHEN w.source_name IN ('trino', 'presto', 'openmetadata', 'amundsen', 'dbt') THEN 'federated' ELSE 'native' END,
    jsonb_strip_nulls(jsonb_build_object(
        'name', a.name, 'type', a.type, 'providers', to_jsonb(a.providers), 'description', a.description,
        'metadata', a.metadata, 'schema', a.schema, 'tags', to_jsonb(a.tags), 'external_links', a.external_links,
        'query', a.query, 'query_language', a.query_language))
FROM (
    SELECT DISTINCT ON (r.pipeline_name, r.source_name) r.id, r.pipeline_name, r.source_name,
           'run:' || r.pipeline_name || '/' || r.source_name AS writer
    FROM runs r
    WHERE r.status = 'completed' AND EXISTS (SELECT 1 FROM run_checkpoints c WHERE c.run_id = r.id)
    ORDER BY r.pipeline_name, r.source_name, r.completed_at DESC
) w
JOIN run_checkpoints c ON c.run_id = w.id AND c.entity_type = 'asset' AND c.operation <> 'deleted'
JOIN assets a ON a.mrn = c.entity_mrn;

-- Everything no pipeline claims was made by hand, through the API or Terraform, or is an OpenLineage stub. Those were never removed as stale and still are not.
INSERT INTO asset_contributions (asset_id, writer, role, fields)
SELECT a.id,
       CASE WHEN a.is_stub THEN 'openlineage' ELSE 'legacy' END,
       CASE WHEN a.is_stub THEN 'reference' ELSE 'manual' END,
       jsonb_strip_nulls(jsonb_build_object(
           'name', a.name, 'type', a.type, 'providers', to_jsonb(a.providers), 'description', a.description,
           'metadata', a.metadata, 'schema', a.schema, 'tags', to_jsonb(a.tags), 'external_links', a.external_links,
           'query', a.query, 'query_language', a.query_language))
FROM assets a
WHERE NOT EXISTS (SELECT 1 FROM asset_contributions c WHERE c.asset_id = a.id);

-- Zone bindings: which zone a host belongs to. Plugins report the hosts they connect to and admins can pin them, so a Kafka Connect sink or an OpenLineage event that only names a host lands on the right system's assets.
CREATE TABLE zone_bindings (
    provider TEXT NOT NULL,
    host TEXT NOT NULL,
    zone TEXT NOT NULL DEFAULT '',
    origin VARCHAR(16) NOT NULL DEFAULT 'discovered',
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (provider, host),
    CHECK (origin IN ('discovered', 'manual'))
);

-- marmot_resolve_asset_mrn is the one place an incoming asset MRN is turned into the MRN of the asset it names: legacy strings get the asset kind, a host hint becomes the zone that host is bound to (or the default zone), and an MRN the asset used to have leads to its current one. An MRN nothing answers to comes back canonical, for the caller to create or reject.
CREATE OR REPLACE FUNCTION marmot_resolve_asset_mrn(m text) RETURNS text LANGUAGE plpgsql STABLE AS $$
DECLARE
    c text;
    parts text[];
    prov text;
    hint text;
    host_only text;
    inst text;
    found_mrn text;
BEGIN
    c := marmot_canonical_asset_mrn(m);
    IF c IS NULL OR left(c, 12) <> 'mrn://asset/' THEN
        RETURN c;
    END IF;
    parts := string_to_array(substr(c, 13), '/');
    IF array_length(parts, 1) = 3 AND strpos(parts[2], '@host=') > 0 THEN
        prov := split_part(parts[2], '@host=', 1);
        hint := split_part(parts[2], '@host=', 2);
        host_only := split_part(hint, ':', 1);
        SELECT b.zone INTO inst FROM zone_bindings b
        WHERE b.provider = prov AND (b.host = hint OR b.host = host_only OR split_part(b.host, ':', 1) = host_only)
        ORDER BY (b.host = hint) DESC, (b.origin = 'manual') DESC, b.updated_at DESC
        LIMIT 1;
        c := 'mrn://asset/' || parts[1] || '/' || prov || CASE WHEN COALESCE(inst, '') <> '' THEN '@' || inst ELSE '' END || '/' || parts[3];
    END IF;
    IF EXISTS (SELECT 1 FROM assets WHERE mrn = c) THEN
        RETURN c;
    END IF;
    SELECT a.mrn INTO found_mrn FROM mrn_aliases al JOIN assets a ON a.id = al.entity_id WHERE al.alias = c AND al.kind = 'asset';
    IF found_mrn IS NOT NULL THEN
        RETURN found_mrn;
    END IF;
    RETURN c;
END
$$;

-- marmot_merge_assets folds one asset into another when both turn out to be the same thing, such as an OpenLineage stub and the table a plugin later found, or a federated copy and the native one. Everything that points at the loser by id or MRN moves to the winner; where the winner already has the same row (the same owner, term or edge) the loser's copy is dropped. The loser's MRN and aliases become aliases of the winner.
CREATE OR REPLACE FUNCTION marmot_merge_assets(loser text, winner text) RETURNS void LANGUAGE plpgsql AS $$
DECLARE
    loser_mrn text;
    winner_mrn text;
    fk record;
    r record;
BEGIN
    IF loser = winner THEN
        RETURN;
    END IF;
    SELECT mrn INTO loser_mrn FROM assets WHERE id = loser;
    SELECT mrn INTO winner_mrn FROM assets WHERE id = winner;
    IF loser_mrn IS NULL OR winner_mrn IS NULL THEN
        RAISE EXCEPTION 'merge: asset not found' USING ERRCODE = 'no_data_found';
    END IF;

    FOR fk IN
        SELECT c.conrelid::regclass AS tbl, a.attname AS col
        FROM pg_constraint c
        JOIN pg_attribute a ON a.attrelid = c.conrelid AND a.attnum = c.conkey[1]
        WHERE c.contype = 'f' AND c.confrelid = 'assets'::regclass AND array_length(c.conkey, 1) = 1
          AND (SELECT attname FROM pg_attribute WHERE attrelid = c.confrelid AND attnum = c.confkey[1]) = 'id'
    LOOP
        FOR r IN EXECUTE format('SELECT ctid FROM %s WHERE %I = $1', fk.tbl, fk.col) USING loser LOOP
            BEGIN
                EXECUTE format('UPDATE %s SET %I = $1 WHERE ctid = $2', fk.tbl, fk.col) USING winner, r.ctid;
            EXCEPTION WHEN unique_violation THEN
                EXECUTE format('DELETE FROM %s WHERE ctid = $1', fk.tbl) USING r.ctid;
            END;
        END LOOP;
    END LOOP;

    FOR r IN SELECT id FROM lineage_edges WHERE source_mrn = loser_mrn OR target_mrn = loser_mrn LOOP
        BEGIN
            UPDATE lineage_edges SET
                source_mrn = CASE WHEN source_mrn = loser_mrn THEN winner_mrn ELSE source_mrn END,
                target_mrn = CASE WHEN target_mrn = loser_mrn THEN winner_mrn ELSE target_mrn END
            WHERE id = r.id;
        EXCEPTION WHEN unique_violation THEN
            DELETE FROM lineage_edges WHERE id = r.id;
        END;
    END LOOP;
    DELETE FROM lineage_edges WHERE source_mrn = winner_mrn AND target_mrn = winner_mrn;
    UPDATE lineage_edges SET job_mrn = winner_mrn WHERE job_mrn = loser_mrn;

    UPDATE doc_pages SET entity_id = winner_mrn WHERE entity_type = 'asset' AND entity_id = loser_mrn;
    FOR r IN SELECT id FROM documentation WHERE mrn = loser_mrn LOOP
        BEGIN
            UPDATE documentation SET mrn = winner_mrn WHERE id = r.id;
        EXCEPTION WHEN unique_violation THEN
            DELETE FROM documentation WHERE id = r.id;
        END;
    END LOOP;

    IF to_regclass('recent_views') IS NOT NULL THEN
        BEGIN
            EXECUTE 'UPDATE recent_views SET item_id = $1 WHERE kind = ''asset'' AND item_id = $2' USING winner, loser;
        EXCEPTION WHEN unique_violation OR undefined_column THEN
            NULL;
        END;
    END IF;

    UPDATE mrn_aliases SET entity_id = winner WHERE kind = 'asset' AND entity_id = loser;
    INSERT INTO mrn_aliases (alias, kind, entity_id, reason) VALUES (loser_mrn, 'asset', winner, 'merged')
    ON CONFLICT (alias) DO UPDATE SET entity_id = EXCLUDED.entity_id, kind = EXCLUDED.kind, reason = EXCLUDED.reason, created_at = NOW();

    DELETE FROM assets WHERE id = loser;
END
$$;

-- Glossary terms: mrn://glossary-term/<source>/<path>. Source is "marmot" for terms made in Marmot and the plugin's source for synced ones; slug is the term's own path level, made unique among its live siblings.
ALTER TABLE glossary_terms ADD COLUMN source VARCHAR(255) NOT NULL DEFAULT 'marmot';
ALTER TABLE glossary_terms ADD COLUMN slug TEXT;
ALTER TABLE glossary_terms ADD COLUMN mrn TEXT;

CREATE OR REPLACE FUNCTION marmot_refresh_glossary_mrns(root uuid) RETURNS void LANGUAGE plpgsql AS $$
DECLARE
    t record;
    base text;
    candidate text;
    n int;
    new_mrn text;
BEGIN
    -- Slugs top-down so a child's siblings are settled before it is.
    FOR t IN
        WITH RECURSIVE sub AS (
            SELECT id, parent_term_id, 0 AS depth FROM glossary_terms WHERE id = root
            UNION ALL
            SELECT g.id, g.parent_term_id, s.depth + 1 FROM glossary_terms g JOIN sub s ON g.parent_term_id = s.id WHERE s.depth < 100
        )
        SELECT g.id, g.name, g.source, g.parent_term_id, g.deleted_at, g.created_at FROM sub JOIN glossary_terms g ON g.id = sub.id
        ORDER BY sub.depth, g.created_at, g.id
    LOOP
        base := marmot_mrn_sanitize(t.name);
        IF base = '' THEN base := 'term'; END IF;
        candidate := base;
        n := 2;
        WHILE t.deleted_at IS NULL AND EXISTS (
            SELECT 1 FROM glossary_terms o
            WHERE o.id <> t.id AND o.deleted_at IS NULL AND o.source = t.source
              AND o.parent_term_id IS NOT DISTINCT FROM t.parent_term_id AND o.slug = candidate
              AND (o.created_at, o.id::text) < (t.created_at, t.id::text)
        ) LOOP
            candidate := base || '-' || n;
            n := n + 1;
        END LOOP;
        UPDATE glossary_terms SET slug = candidate WHERE id = t.id AND slug IS DISTINCT FROM candidate;
    END LOOP;

    FOR t IN
        WITH RECURSIVE sub AS (
            SELECT id FROM glossary_terms WHERE id = root
            UNION ALL
            SELECT g.id FROM glossary_terms g JOIN sub s ON g.parent_term_id = s.id
        ),
        paths AS (
            SELECT g.id, ARRAY[g.slug] AS path, g.parent_term_id AS next, 0 AS depth
            FROM glossary_terms g WHERE g.id IN (SELECT id FROM sub)
            UNION ALL
            SELECT p.id, ARRAY[a.slug] || p.path, a.parent_term_id, p.depth + 1
            FROM paths p JOIN glossary_terms a ON a.id = p.next WHERE p.depth < 100
        )
        SELECT p.id, p.path, g.source, g.mrn AS old_mrn FROM paths p JOIN glossary_terms g ON g.id = p.id WHERE p.next IS NULL
    LOOP
        new_mrn := 'mrn://glossary-term/' || marmot_mrn_sanitize_segment(t.source) || '/' || marmot_mrn_join_path(t.path);
        IF t.old_mrn IS DISTINCT FROM new_mrn THEN
            UPDATE glossary_terms SET mrn = new_mrn WHERE id = t.id;
            IF t.old_mrn IS NOT NULL THEN
                INSERT INTO mrn_aliases (alias, kind, entity_id, reason) VALUES (t.old_mrn, 'glossary-term', t.id::text, 'renamed')
                ON CONFLICT (alias) DO UPDATE SET entity_id = EXCLUDED.entity_id, kind = EXCLUDED.kind, reason = EXCLUDED.reason, created_at = NOW();
            END IF;
        END IF;
    END LOOP;
END
$$;

CREATE OR REPLACE FUNCTION marmot_glossary_mrn_trigger() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    -- Siblings keep the suffixes they were given when this term leaves, is deleted or is renamed: an MRN that changes on its own is worse than one with a number on the end.
    PERFORM marmot_refresh_glossary_mrns(NEW.id);
    RETURN NULL;
END
$$;

CREATE TRIGGER glossary_terms_mrn AFTER INSERT OR UPDATE OF name, parent_term_id, source, deleted_at ON glossary_terms
    FOR EACH ROW EXECUTE FUNCTION marmot_glossary_mrn_trigger();

DO $$
DECLARE
    r record;
BEGIN
    FOR r IN SELECT id FROM glossary_terms WHERE parent_term_id IS NULL ORDER BY created_at, id LOOP
        PERFORM marmot_refresh_glossary_mrns(r.id);
    END LOOP;
    -- Orphans whose parent row is gone still get an MRN.
    FOR r IN SELECT id FROM glossary_terms WHERE mrn IS NULL ORDER BY created_at, id LOOP
        PERFORM marmot_refresh_glossary_mrns(r.id);
    END LOOP;
END
$$;

CREATE UNIQUE INDEX idx_glossary_terms_mrn ON glossary_terms (mrn) WHERE deleted_at IS NULL;

-- Data products: mrn://data-product/<slug>, unique even when two names slug the same.
ALTER TABLE data_products ADD COLUMN mrn TEXT;

CREATE OR REPLACE FUNCTION marmot_data_product_mrn_trigger() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE
    base text;
    candidate text;
    n int := 2;
BEGIN
    IF TG_OP = 'UPDATE' AND OLD.name IS NOT DISTINCT FROM NEW.name AND NEW.mrn IS NOT NULL THEN
        RETURN NEW;
    END IF;
    base := 'mrn://data-product/' || COALESCE(NULLIF(marmot_mrn_slug(NEW.name), ''), 'product');
    candidate := base;
    WHILE EXISTS (SELECT 1 FROM data_products o WHERE o.id <> NEW.id AND o.mrn = candidate) LOOP
        candidate := base || '-' || n;
        n := n + 1;
    END LOOP;
    IF TG_OP = 'UPDATE' AND OLD.mrn IS NOT NULL AND OLD.mrn <> candidate THEN
        INSERT INTO mrn_aliases (alias, kind, entity_id, reason) VALUES (OLD.mrn, 'data-product', NEW.id::text, 'renamed')
        ON CONFLICT (alias) DO UPDATE SET entity_id = EXCLUDED.entity_id, kind = EXCLUDED.kind, reason = EXCLUDED.reason, created_at = NOW();
    END IF;
    NEW.mrn := candidate;
    RETURN NEW;
END
$$;

CREATE TRIGGER data_products_mrn BEFORE INSERT OR UPDATE OF name, mrn ON data_products
    FOR EACH ROW EXECUTE FUNCTION marmot_data_product_mrn_trigger();

DO $$
DECLARE
    r record;
BEGIN
    FOR r IN SELECT id FROM data_products ORDER BY created_at, id LOOP
        UPDATE data_products SET name = name WHERE id = r.id;
    END LOOP;
END
$$;
CREATE UNIQUE INDEX idx_data_products_mrn ON data_products (mrn);

---- create above / drop below ----

-- Asset MRNs are not rewritten back to the legacy form: run a release before the kinded format only against a backup taken before upgrading.
DROP TRIGGER IF EXISTS data_products_mrn ON data_products;
DROP FUNCTION IF EXISTS marmot_data_product_mrn_trigger();
DROP INDEX IF EXISTS idx_data_products_mrn;
ALTER TABLE data_products DROP COLUMN IF EXISTS mrn;
DROP TRIGGER IF EXISTS glossary_terms_mrn ON glossary_terms;
DROP FUNCTION IF EXISTS marmot_glossary_mrn_trigger();
DROP FUNCTION IF EXISTS marmot_refresh_glossary_mrns(uuid);
DROP INDEX IF EXISTS idx_glossary_terms_mrn;
ALTER TABLE glossary_terms DROP COLUMN IF EXISTS mrn, DROP COLUMN IF EXISTS slug, DROP COLUMN IF EXISTS source;
DROP FUNCTION IF EXISTS marmot_merge_assets(text, text);
DROP FUNCTION IF EXISTS marmot_resolve_asset_mrn(text);
DROP TABLE IF EXISTS zone_bindings;
DROP INDEX IF EXISTS idx_runs_pipeline_source_owner;
ALTER TABLE runs DROP COLUMN IF EXISTS owner;
DROP TABLE IF EXISTS asset_contributions;
DROP TRIGGER IF EXISTS assets_drop_aliases ON assets;
DROP FUNCTION IF EXISTS marmot_drop_asset_aliases();
DROP TABLE IF EXISTS mrn_aliases;
DROP TRIGGER IF EXISTS run_entities_canonical_mrn ON run_entities;
DROP TRIGGER IF EXISTS run_checkpoints_canonical_mrn ON run_checkpoints;
DROP TRIGGER IF EXISTS doc_pages_canonical_mrn ON doc_pages;
DROP TRIGGER IF EXISTS documentation_canonical_mrn ON documentation;
DROP TRIGGER IF EXISTS lineage_edges_canonical_mrn ON lineage_edges;
DROP TRIGGER IF EXISTS assets_canonical_mrn ON assets;
DROP FUNCTION IF EXISTS marmot_canonicalise_asset_mrn_column();
ALTER TABLE assets DROP CONSTRAINT IF EXISTS assets_mrn_kinded;
