The OpenMetadata plugin imports an entire OpenMetadata instance in one run: tables, views, stored procedures, topics, buckets, dashboards, charts, pipelines, models, search indices, API endpoints, drive files and spreadsheets, the business glossary, and the lineage between them.

OpenMetadata is a catalog, so everything in it describes something that lives somewhere else. This plugin catalogues each entity as the technology it belongs to rather than as an OpenMetadata thing: a table under a Postgres service becomes a PostgreSQL asset in Marmot, addressed exactly as Marmot's own PostgreSQL plugin would address it.

## Supported OpenMetadata Versions

OpenMetadata 1.4 or newer. The plugin negotiates the field list with the server on the first call to each endpoint: fields the server does not know are dropped from later requests, so one plugin binary works across every version in that range. Entity kinds a server predates (drives, API collections, and so on) are skipped without failing the run.

## Getting a Token

The plugin authenticates as a bot or as a user, with a JWT.

For a bot, open **Settings → Bots** in OpenMetadata, pick a bot such as `ingestion-bot`, and copy its token. For a user, open **Settings → Members**, pick the user, and create a personal access token. The token needs read access to the entities you want to import.

## Cutting Over from OpenMetadata

Moving off OpenMetadata is not a single switch, so this plugin is built to run on a schedule for as long as the move takes. Each run brings across whatever changed in OpenMetadata; re-running is safe, and assets that have not changed are left alone.

Anything written in Marmot survives every re-sync. A description edited in Marmot is stored separately from the one the run imported, so the next sync refreshes the imported side and never overwrites the edit. The same holds for tags, owners and glossary terms added in Marmot.

When you are ready to catalogue a system directly, add its own pipeline (for example the PostgreSQL plugin against the database OpenMetadata was describing). Imported assets and native ones share an MRN, so the native run takes over the assets that already exist instead of creating a second copy. Nothing needs to be deleted or re-pointed.

When you are done, stop scheduling the run. The imported assets stay exactly as they are. Do not use `marmot ingest --destroy`: it deletes every asset the pipeline ever created, including ones another pipeline has since taken over.

## Running Alongside Native Plugins

By default an imported asset lands on the same MRN the technology's native Marmot plugin would use, so the two runs contribute to one asset instead of creating two. A Postgres table becomes `mrn://table/postgresql/orders` whether Marmot read it from OpenMetadata or from the database itself.

This means names drop the levels the native plugin does not use. Marmot's own plugins for Postgres, MySQL, BigQuery, MongoDB, ClickHouse, Glue and Iceberg all name a table by its bare name, so `public.orders` and `staging.orders` resolve to one asset, as do two OpenMetadata services holding the same table name. Set `naming: qualified` to keep them apart instead, at the cost of no longer merging with native runs.

Technologies Marmot has no plugin for yet, such as Snowflake or Looker, are imported under their own provider name. Nothing is invented: an entity is only imported when Marmot already has an asset type that means the same thing.

## Container Prefixes and Drives

Object storage comes across as the bucket alone. OpenMetadata models the prefixes inside a bucket as containers of their own, but Marmot's S3, GCS and Azure Blob plugins catalogue the bucket and nothing below it, so an imported prefix would sit in the catalog forever without a native run ever updating it. Set `include_container_prefixes: true` to import the hierarchy anyway, worth doing when nothing else is going to catalogue that bucket.

Drives are different: a drive really is a tree of folders and Marmot's GoogleDrive plugin catalogues it as one, so folders come across in full.
