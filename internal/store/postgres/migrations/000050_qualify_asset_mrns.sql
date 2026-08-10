-- Qualify the MRNs of assets whose plugin used to identify them by a bare
-- name, and move assets whose provider word has since been corrected.
--
-- PostgreSQL, MySQL, BigQuery and MongoDB each iterate a parent level --
-- a schema, a database, a dataset -- but built the MRN from the object
-- name alone. Two real objects that share a name therefore collapsed onto
-- one asset, and the second silently overwrote the first. On OpenMetadata's
-- own sample data that merges 250 Snowflake-shaped tables into 10.
--
-- The plugins now put the parent in the MRN. This migration moves existing
-- rows to the same identity so an upgrade does not strand them as
-- duplicates. It rebuilds each MRN from metadata the plugins already
-- stored, so no re-ingestion is needed.
--
-- Nothing else has to move: owners, tags, subscriptions, data product
-- membership, run history and documentation all reference assets.id, which
-- is untouched. Only lineage_edges references assets.mrn, and it is
-- rewritten here in step with the assets it points at.

BEGIN;

-- The foreign keys are NO ACTION, so the assets and the edges that point at
-- them cannot be updated one after the other. Drop them for the rewrite and
-- put them back exactly as they were.
ALTER TABLE lineage_edges DROP CONSTRAINT IF EXISTS lineage_edges_source_mrn_fkey;
ALTER TABLE lineage_edges DROP CONSTRAINT IF EXISTS lineage_edges_target_mrn_fkey;

CREATE TEMP TABLE mrn_rewrite ON COMMIT DROP AS
WITH
-- Pass 1: rows still addressed by a bare object name, moved onto the
-- qualified identity their plugin now declares. Only the name component of
-- the MRN changes; the type and service stay as they were.
qualified AS (
    SELECT
        a.id,
        a.mrn AS old_mrn,
        'mrn://'
            || split_part(replace(a.mrn, 'mrn://', ''), '/', 1)
            || '/'
            || split_part(replace(a.mrn, 'mrn://', ''), '/', 2)
            || '/'
            || lower(replace(replace(parent.value || '.' || a.name, '/', '-'), ' ', '-'))
            AS new_mrn,
        NULL::text AS new_provider,
        NULL::text AS new_type
    FROM assets a
    CROSS JOIN LATERAL (
        -- The parent level is written under different keys depending on
        -- which plugin produced the asset. OpenMetadata always writes it
        -- as "schema"; the technology's own plugin uses its own word. Take
        -- "schema" first so an OpenMetadata import resolves correctly, and
        -- fall back to the native key. NULLIF keeps OpenMetadata's
        -- "default" placeholder out of the identity.
        SELECT CASE a.providers[1]
            -- PostgreSQL walks every database on the server, so the
            -- database belongs in the identity too.
            WHEN 'PostgreSQL' THEN nullif(a.metadata->>'database', 'default')
                                   || '.' || nullif(a.metadata->>'schema', 'default')
            WHEN 'MySQL'      THEN nullif(a.metadata->>'schema', 'default')
            WHEN 'BigQuery'   THEN coalesce(nullif(a.metadata->>'schema', 'default'),
                                            a.metadata->>'dataset_id')
            WHEN 'MongoDB'    THEN coalesce(nullif(a.metadata->>'schema', 'default'),
                                            a.metadata->>'database')
            -- These declare a qualified MRN of their own. Their identity only
            -- started reaching the server once the ingest API carried the MRN,
            -- so rows written before that are still bare.
            --
            -- Each native key below is the word that plugin actually writes.
            -- Reading only "schema" left these three branches dead for every
            -- natively ingested row: plugins/clickhouse writes "database",
            -- plugins/glue writes "database_name", and plugins/iceberg writes
            -- "namespace" on its tables and views.
            WHEN 'ClickHouse' THEN coalesce(nullif(a.metadata->>'schema', 'default'),
                                            a.metadata->>'database')
            WHEN 'Glue'       THEN coalesce(nullif(a.metadata->>'schema', 'default'),
                                            a.metadata->>'database_name')
            WHEN 'Iceberg'    THEN coalesce(nullif(a.metadata->>'schema', 'default'),
                                            a.metadata->>'namespace')
            -- plugins/duckdb identifies a table as schema.table while showing
            -- the bare name. There is no OpenMetadata DuckDB service, so no
            -- "default" placeholder can appear.
            WHEN 'DuckDB'     THEN a.metadata->>'schema'

            -- One-level parents. Every one of these is reachable through
            -- plugins/trino, which writes metadata["schema"] and displays the
            -- bare table name, and through OpenMetadata, which also writes
            -- "schema".
            WHEN 'Oracle'     THEN nullif(a.metadata->>'schema', 'default')
            WHEN 'Cassandra'  THEN nullif(a.metadata->>'schema', 'default')
            WHEN 'Teradata'   THEN nullif(a.metadata->>'schema', 'default')
            WHEN 'Exasol'     THEN nullif(a.metadata->>'schema', 'default')
            WHEN 'Impala'     THEN nullif(a.metadata->>'schema', 'default')
            WHEN 'Doris'      THEN nullif(a.metadata->>'schema', 'default')
            WHEN 'StarRocks'  THEN nullif(a.metadata->>'schema', 'default')
            WHEN 'Druid'      THEN nullif(a.metadata->>'schema', 'default')
            WHEN 'SAS'        THEN nullif(a.metadata->>'schema', 'default')
            WHEN 'Accumulo'   THEN nullif(a.metadata->>'schema', 'default')
            WHEN 'Phoenix'    THEN nullif(a.metadata->>'schema', 'default')
            WHEN 'Ignite'     THEN nullif(a.metadata->>'schema', 'default')
            WHEN 'Kudu'       THEN nullif(a.metadata->>'schema', 'default')
            -- Bigtable's container is the instance, which OpenMetadata puts
            -- at the schema level. The GCP project stays out of the identity
            -- for the same reason it does in BigQuery.
            WHEN 'Bigtable'   THEN nullif(a.metadata->>'schema', 'default')

            -- Two-level parents (database.schema), written by OpenMetadata as
            -- separate keys.
            --
            -- Snowflake, Redshift and SQL Server have two legitimate shapes:
            -- plugins/trino names them schema.table because a Trino catalog
            -- binds to one database, while the OpenMetadata projection names
            -- them database.schema.table because OpenMetadata reads whole
            -- accounts. Branch on the writer: a Trino row carries "catalog",
            -- an OpenMetadata row does not.
            WHEN 'Snowflake'  THEN CASE
                WHEN a.metadata ? 'catalog' THEN nullif(a.metadata->>'schema', 'default')
                ELSE concat_ws('.', nullif(a.metadata->>'database', 'default'),
                                    nullif(a.metadata->>'schema', 'default'))
            END
            WHEN 'Redshift'   THEN CASE
                WHEN a.metadata ? 'catalog' THEN nullif(a.metadata->>'schema', 'default')
                ELSE concat_ws('.', nullif(a.metadata->>'database', 'default'),
                                    nullif(a.metadata->>'schema', 'default'))
            END
            WHEN 'SQL Server' THEN CASE
                WHEN a.metadata ? 'catalog' THEN nullif(a.metadata->>'schema', 'default')
                ELSE concat_ws('.', nullif(a.metadata->>'database', 'default'),
                                    nullif(a.metadata->>'schema', 'default'))
            END

            WHEN 'Databricks'       THEN concat_ws('.', nullif(a.metadata->>'database', 'default'),
                                                        nullif(a.metadata->>'schema', 'default'))
            WHEN 'Hive'             THEN concat_ws('.', nullif(a.metadata->>'database', 'default'),
                                                        nullif(a.metadata->>'schema', 'default'))
            WHEN 'Db2'              THEN concat_ws('.', nullif(a.metadata->>'database', 'default'),
                                                        nullif(a.metadata->>'schema', 'default'))
            WHEN 'Vertica'          THEN concat_ws('.', nullif(a.metadata->>'database', 'default'),
                                                        nullif(a.metadata->>'schema', 'default'))
            WHEN 'Greenplum'        THEN concat_ws('.', nullif(a.metadata->>'database', 'default'),
                                                        nullif(a.metadata->>'schema', 'default'))
            WHEN 'CockroachDB'      THEN concat_ws('.', nullif(a.metadata->>'database', 'default'),
                                                        nullif(a.metadata->>'schema', 'default'))
            WHEN 'Azure Synapse'    THEN concat_ws('.', nullif(a.metadata->>'database', 'default'),
                                                        nullif(a.metadata->>'schema', 'default'))
            WHEN 'Microsoft Fabric' THEN concat_ws('.', nullif(a.metadata->>'database', 'default'),
                                                        nullif(a.metadata->>'schema', 'default'))
            WHEN 'SAP HANA'         THEN concat_ws('.', nullif(a.metadata->>'database', 'default'),
                                                        nullif(a.metadata->>'schema', 'default'))
            WHEN 'IOMETE'           THEN concat_ws('.', nullif(a.metadata->>'database', 'default'),
                                                        nullif(a.metadata->>'schema', 'default'))
            WHEN 'Dremio'           THEN concat_ws('.', nullif(a.metadata->>'database', 'default'),
                                                        nullif(a.metadata->>'schema', 'default'))
            WHEN 'Couchbase'        THEN concat_ws('.', nullif(a.metadata->>'database', 'default'),
                                                        nullif(a.metadata->>'schema', 'default'))
            WHEN 'Trino'            THEN concat_ws('.', nullif(a.metadata->>'database', 'default'),
                                                        nullif(a.metadata->>'schema', 'default'))
            WHEN 'Presto'           THEN concat_ws('.', nullif(a.metadata->>'database', 'default'),
                                                        nullif(a.metadata->>'schema', 'default'))
        END AS value
    ) parent
    WHERE a.providers[1] IN ('PostgreSQL', 'MySQL', 'BigQuery', 'MongoDB',
                             'ClickHouse', 'Glue', 'Iceberg', 'DuckDB',
                             'Oracle', 'Cassandra', 'Teradata', 'Exasol',
                             'Impala', 'Doris', 'StarRocks', 'Druid', 'SAS',
                             'Accumulo', 'Phoenix', 'Ignite', 'Kudu', 'Bigtable',
                             'Snowflake', 'Redshift', 'SQL Server', 'Databricks',
                             'Hive', 'Db2', 'Vertica', 'Greenplum', 'CockroachDB',
                             'Azure Synapse', 'Microsoft Fabric', 'SAP HANA',
                             'IOMETE', 'Dremio', 'Couchbase', 'Trino', 'Presto')
      AND a.type IN ('Table', 'View', 'Collection', 'ExternalTable')
      AND parent.value IS NOT NULL
      AND parent.value <> ''
      -- only rows still carrying the old bare identity
      AND split_part(replace(a.mrn, 'mrn://', ''), '/', 3) = lower(replace(replace(a.name, '/', '-'), ' ', '-'))
),

-- Pass 2: rows whose provider word itself was wrong, so the service
-- component of the MRN has to move. These rows are already qualified, so
-- pass 1 never sees them; left alone they would sit beside the assets the
-- corrected plugin now writes instead of merging with them.
renamed AS (
    SELECT
        a.id,
        a.mrn AS old_mrn,
        'mrn://'
            || rewrite.new_type_slug
            || '/'
            || rewrite.new_service
            || '/'
            || rewrite.new_name
            AS new_mrn,
        rewrite.new_provider,
        rewrite.new_type
    FROM assets a
    CROSS JOIN LATERAL (
        SELECT
            CASE a.providers[1]
                -- Athena keeps no catalog of its own: its tables are Glue Data
                -- Catalog tables and plugins/glue already catalogues them, so
                -- the import now files them under Glue.
                WHEN 'Athena'           THEN 'Glue'
                -- dbt's own provider words, corrected to the ones the native
                -- plugins pass to mrn.New.
                WHEN 'Postgres'         THEN 'PostgreSQL'
                WHEN 'AWS Glue'         THEN 'Glue'
                WHEN 'Azure Synapse'    THEN 'Azure Synapse'
                WHEN 'Microsoft Fabric' THEN 'Microsoft Fabric'
            END AS new_provider,
            CASE a.providers[1]
                WHEN 'Athena'           THEN 'glue'
                WHEN 'Postgres'         THEN 'postgresql'
                WHEN 'AWS Glue'         THEN 'glue'
                -- A space in the service would land in the MRN and in every
                -- URL built from it, so it is dropped the way the plugins'
                -- mrnService helper drops it.
                WHEN 'Azure Synapse'    THEN 'azuresynapse'
                WHEN 'Microsoft Fabric' THEN 'microsoftfabric'
            END AS new_service,
            -- A materialized view is catalogued as a plain View, the way
            -- plugins/postgresql and plugins/clickhouse already do it. Any
            -- other compound type keeps its words but loses the space.
            CASE
                WHEN lower(a.type) = 'materialized view' THEN 'View'
                ELSE a.type
            END AS new_type,
            CASE
                WHEN lower(a.type) = 'materialized view' THEN 'view'
                ELSE replace(lower(a.type), ' ', '')
            END AS new_type_slug,
            CASE
                -- Athena rows already carry the schema.table name plugins/glue
                -- uses, so only the service moves.
                WHEN a.providers[1] = 'Athena' THEN split_part(replace(a.mrn, 'mrn://', ''), '/', 3)
                -- dbt addressed every table as database.schema.table. The
                -- plugin that owns the table keeps fewer levels.
                WHEN a.providers[1] IN ('Postgres', 'AWS Glue')
                     AND a.metadata->>'schema' IS NOT NULL
                     AND a.metadata->>'table_name' IS NOT NULL
                    THEN lower(replace(replace(a.metadata->>'schema' || '.' || (a.metadata->>'table_name'), '/', '-'), ' ', '-'))
                -- Synapse and Fabric keep database.schema.table; only the
                -- spaced service changes.
                ELSE split_part(replace(a.mrn, 'mrn://', ''), '/', 3)
            END AS new_name
    ) rewrite
    WHERE a.providers[1] IN ('Athena', 'Postgres', 'AWS Glue', 'Azure Synapse', 'Microsoft Fabric')
      -- Azure Synapse and Microsoft Fabric are also OpenMetadata provider
      -- words. Only dbt-written rows need moving; an OpenMetadata row is
      -- already on the slugged service because the projection slugs it.
      AND (a.providers[1] NOT IN ('Azure Synapse', 'Microsoft Fabric')
           OR split_part(replace(a.mrn, 'mrn://', ''), '/', 2) LIKE '% %')
      AND rewrite.new_service IS NOT NULL
      AND rewrite.new_name <> ''
),

combined AS (
    SELECT * FROM qualified
    UNION ALL
    SELECT * FROM renamed
),

-- A row can only move once. Nothing currently qualifies under both passes,
-- but deduplicating keeps that from becoming a silent double rewrite.
deduped AS (
    SELECT DISTINCT ON (id) id, old_mrn, new_mrn, new_provider, new_type
    FROM combined
    ORDER BY id
)
-- assets.mrn is UNIQUE, and this runs inside one transaction that the
-- migrator propagates out of Initialize, so a single duplicate would stop
-- the server from starting. Two rows can compute the same new MRN when
-- their old names differed only by something mrn.New folds away: case, or
-- a slash against a hyphen. Keep the oldest of any such group and leave
-- the rest on their existing MRN rather than aborting the upgrade.
SELECT DISTINCT ON (new_mrn) id, old_mrn, new_mrn, new_provider, new_type
FROM deduped
WHERE new_mrn <> old_mrn
  -- never move a row onto an MRN that is already taken
  AND NOT EXISTS (SELECT 1 FROM assets other WHERE other.mrn = deduped.new_mrn)
ORDER BY new_mrn, id;

UPDATE lineage_edges e
SET source_mrn = r.new_mrn
FROM mrn_rewrite r
WHERE e.source_mrn = r.old_mrn;

UPDATE lineage_edges e
SET target_mrn = r.new_mrn
FROM mrn_rewrite r
WHERE e.target_mrn = r.old_mrn;

UPDATE assets a
SET mrn = r.new_mrn,
    -- Keep the displayed provider and type in step with the identity, or
    -- the row reads as one technology and is addressed as another.
    providers = CASE
        WHEN r.new_provider IS NULL THEN a.providers
        ELSE array_prepend(r.new_provider, a.providers[2:])
    END,
    type = coalesce(r.new_type, a.type)
FROM mrn_rewrite r
WHERE a.id = r.id;

UPDATE search_index s
SET mrn = r.new_mrn
FROM mrn_rewrite r
WHERE s.mrn = r.old_mrn;

UPDATE documentation d
SET mrn = r.new_mrn
FROM mrn_rewrite r
WHERE d.mrn = r.old_mrn;

UPDATE run_checkpoints c
SET entity_mrn = r.new_mrn
FROM mrn_rewrite r
WHERE c.entity_mrn = r.old_mrn;

UPDATE run_entities n
SET entity_mrn = r.new_mrn
FROM mrn_rewrite r
WHERE n.entity_mrn = r.old_mrn;

ALTER TABLE lineage_edges
    ADD CONSTRAINT lineage_edges_source_mrn_fkey
    FOREIGN KEY (source_mrn) REFERENCES assets(mrn);

ALTER TABLE lineage_edges
    ADD CONSTRAINT lineage_edges_target_mrn_fkey
    FOREIGN KEY (target_mrn) REFERENCES assets(mrn);

COMMIT;

---- create above / drop below ----

-- Irreversible by design. The old identity was a bare name, and two objects
-- that shared one were a single row; restoring it would merge them again and
-- lose whichever was written second. Roll forward, never back.
