---
title: OpenMetadata
description: This plugin imports the contents of an OpenMetadata instance, cataloguing every entity as the technology it actually belongs to.
status: experimental
---

# OpenMetadata

<div class="flex flex-col gap-3 mb-6 pb-6 border-b border-gray-200">
<div class="flex items-center gap-3">
<span class="inline-flex items-center rounded-full px-4 py-2 text-sm font-medium bg-earthy-yellow-300 text-earthy-yellow-900">Experimental</span>
</div>
<div class="flex items-center gap-2">
<span class="text-sm text-gray-500">Creates:</span>
<div class="flex flex-wrap gap-2"><span class="inline-flex items-center rounded-lg px-4 py-2 text-sm font-medium bg-earthy-green-100 text-earthy-green-800 border border-earthy-green-300">Assets</span><span class="inline-flex items-center rounded-lg px-4 py-2 text-sm font-medium bg-earthy-green-100 text-earthy-green-800 border border-earthy-green-300">Lineage</span><span class="inline-flex items-center rounded-lg px-4 py-2 text-sm font-medium bg-earthy-green-100 text-earthy-green-800 border border-earthy-green-300">Run History</span><span class="inline-flex items-center rounded-lg px-4 py-2 text-sm font-medium bg-earthy-green-100 text-earthy-green-800 border border-earthy-green-300">Glossary</span></div>
</div>
</div>

import { CalloutCard } from '@site/src/components/DocCard';

<CalloutCard
  title="Configure in the UI"
  description="This plugin can be configured directly in the Marmot UI with a step-by-step wizard."
  href="/docs/Populating/UI"
  buttonText="View Guide"
  variant="secondary"
  icon="mdi:cursor-default-click"
/>

The OpenMetadata plugin imports an entire OpenMetadata instance in one run: tables, views, stored procedures, topics, buckets, dashboards, charts, pipelines, models, search indices, API endpoints, drive files and spreadsheets, the business glossary, and the lineage between them.

OpenMetadata is a catalog, so everything in it describes something that lives somewhere else. The plugin catalogues each entity as the technology it actually describes rather than as an OpenMetadata thing: a table under a Postgres service becomes a PostgreSQL asset in Marmot, addressed exactly as Marmot's own PostgreSQL plugin would address it. Technologies Marmot has no plugin for yet, such as Snowflake or Looker, come across under their own provider name. Point Marmot at OpenMetadata and the result looks like a catalog Marmot built itself.

OpenMetadata 1.4 or newer is supported. The plugin negotiates the field list with the server on the first call to each endpoint, so one plugin binary works across every version in that range, and entity kinds a server predates (drives, API collections, and so on) are skipped without failing the run.

## What it imports

| OpenMetadata | Marmot asset type |
|---|---|
| database or schema, whichever the engine really has | Database, Dataset, Namespace or Catalog |
| table | Table, View, or the name the technology's own plugin uses (a MongoDB collection is a Collection, a BigQuery external table an ExternalTable) |
| stored procedure | Function |
| topic | Topic |
| container | Bucket, or Container on Azure Blob. Only the top level one is imported |
| drive | Drive |
| drive folder | Folder |
| drive file | File |
| spreadsheet | Spreadsheet |
| a sheet of a spreadsheet | Table, with its columns |
| dashboard, chart, dashboard data model | Dashboard, Chart, Data Model Object |
| pipeline and its tasks | Pipeline, Task |
| ML model | Model |
| search index | Table on Elasticsearch and OpenSearch, Index elsewhere |
| API collection and endpoint | Service, Endpoint |
| glossary | Glossary Term, the root of its own vocabulary |
| glossary term | Glossary Term, nested under its parent term or its glossary |
| entity lineage | Lineage, carrying the pipeline that moved the data |
| pipeline executions | Run History |

Descriptions, columns, classification tags, owners, domains and data products come across on each asset, and nothing loses its trail: every asset gets an OpenMetadata link that jumps straight to the entity it was imported from, and an `openmetadata` metadata object carrying the fully qualified name, the service and when the entity last changed. Mid-migration, any asset in Marmot traces back to its source in one click.

The glossary comes across as glossary terms rather than as tags, so a term keeps its definition, its synonyms and the terms below it, and the assets it was curated onto are assigned it. Terms are identified by their OpenMetadata fully qualified name, so two glossaries can each hold a `Customer` without becoming one term. Set `glossary_terms_as_tags: true` to also copy each assigned term onto the asset's tags, or `include_glossary: false` to leave the glossary behind entirely.

Object storage comes across as the bucket alone. OpenMetadata models the prefixes inside a bucket as containers of their own, but Marmot's S3, GCS and Azure Blob plugins catalogue the bucket and nothing below it, so an imported prefix would sit in the catalog forever without a native run ever updating it. Each run reports how many it left out. Set `include_container_prefixes: true` to import the hierarchy anyway, which is worth doing when nothing else is going to catalogue that bucket.

Drives are different and come across in full: a drive really is a tree of folders, and Marmot's GoogleDrive plugin catalogues it as one. The drive itself is catalogued, folders are linked up to its root, and documents are placed by their path rather than their OpenMetadata name, because OpenMetadata files some of them under the service instead of the folder they live in. Folders that exist only in a path are created too, marked `inferred_from_path`, so nothing dangles.

Columns are part of the asset in Marmot rather than entities of their own, so a table with two hundred columns is one asset carrying two hundred columns, not two hundred and one things. That is why OpenMetadata's own totals are far larger than the number of assets an import produces.

Marmot ingestion runs cannot create teams, users, domains or data products as objects of their own, so those stay on the assets as metadata. Data quality test cases and OpenMetadata's own usage analytics have no Marmot equivalent and are not imported.

## The Migration Pattern

A catalog migration fails when it has to happen all at once. This plugin is built to run on a schedule for as long as the move takes, so no single day has to be the day everything switches.

**1. Schedule the import.** Set it up as a recurring pipeline through the UI wizard, the CLI, Terraform, Pulumi or the REST API; the [Populating docs](/docs/Populating/) cover each. Everything is imported by default, and the configuration below covers scoping down to specific services or service types.

**2. Keep working in both catalogs.** Each run brings across whatever changed in OpenMetadata, so the two stay in step while people are still working in both. Re-running is safe: assets that have not changed are left alone. Anything written in Marmot survives every re-sync, because a description edited in Marmot is stored separately from the imported one, so the next sync refreshes the imported side and never overwrites the edit. The same holds for tags, owners and glossary terms added in Marmot.

**3. Adopt native plugins one system at a time.** When you are ready to catalogue a system directly, add its own pipeline, for example the [PostgreSQL plugin](/docs/Plugins/PostgreSQL) against the database OpenMetadata was describing. Imported and native assets share an identity, so the native run takes over the assets that are already there instead of creating a second copy. Nothing needs to be deleted or re-pointed, and the descriptions people wrote stay put.

**4. Switch OpenMetadata off.** When nothing depends on it anymore, stop scheduling the run. The imported assets stay exactly as they are.

<CalloutCard
  title="Stop the schedule, do not destroy the pipeline"
  description="marmot ingest --destroy deletes every asset the pipeline ever created, including ones another pipeline has since taken over. To retire this plugin, remove its schedule and leave the assets in place."
  variant="secondary"
  icon="mdi:alert-outline"
/>

## Running it Alongside Marmot's Own Plugins

By default an imported asset lands on the same MRN the technology's native Marmot plugin would use, so the two runs contribute to one asset instead of creating two. A Postgres table becomes `mrn://table/postgresql/orders` whether Marmot read it from OpenMetadata or from the database itself, so whichever run happens next updates the asset that is already there.

That means names drop the levels the native plugin does not use. Marmot's own plugins for Postgres, MySQL, BigQuery, MongoDB, ClickHouse, Glue and Iceberg all name a table by its bare name, so `public.orders` and `staging.orders` resolve to one asset, as do two OpenMetadata services holding the same table name. The run reports every entity it merged this way. Set `naming: qualified` to keep them apart instead, at the cost of no longer merging with native runs, which also gives up the handover described above:

```yaml
runs:
  - openmetadata:
      host: "https://openmetadata.company.com"
      jwt_token: "eyJraWQiOiJHYjM4OWEtOWY3Ni1nZGpzLWE5..."
      naming: qualified
```

## Getting a Token

The plugin authenticates as a bot or as a user, with a JWT.

For a bot, open **Settings → Bots** in OpenMetadata, pick a bot such as `ingestion-bot`, and copy its token. For a user, open **Settings → Members**, pick the user, and create a personal access token. The token needs read access to the entities you want to import.

## Example Configuration

```yaml

host: "https://openmetadata.company.com"
jwt_token: "eyJraWQiOiJHYjM4OWEtOWY3Ni1nZGpzLWE5..."
exclude_service_types:
  - "Metadata"
tags:
  - "openmetadata"

```

Import a single service:

```yaml

host: "https://openmetadata.company.com"
jwt_token: "eyJraWQiOiJHYjM4OWEtOWY3Ni1nZGpzLWE5..."
services:
  - "postgres_prod"

```

## Configuration
The following configuration options are available:

| Property | Type | Required | Description |
|----------|------|----------|-------------|
| concurrency | int | false | Parallel lineage requests |
| exclude_service_types | []string | false | OpenMetadata service types to skip |
| exclude_services | []string | false | OpenMetadata services to skip |
| external_links | []ExternalLink | false | External links to show on all assets |
| filter | Filter | false | Filter discovered assets by name (regex) |
| glossary_terms_as_tags | bool | false | Also copy assigned glossary terms onto assets as tags. They are imported as glossary terms either way |
| host | string | true | OpenMetadata server URL, for example https://openmetadata.company.com |
| include_apis | bool | false | Import API collections and endpoints |
| include_columns | bool | false | Import column, field and feature definitions |
| include_container_prefixes | bool | false | Also import the prefixes and folders inside a storage container. Marmot's own object storage plugins catalogue only the container itself |
| include_containers | bool | false | Import object storage buckets and containers |
| include_drives | bool | false | Import drive directories, files, spreadsheets and worksheets |
| include_dashboards | bool | false | Import dashboards, charts and dashboard data models |
| include_deleted | bool | false | Import entities OpenMetadata has soft deleted |
| include_glossary | bool | false | Import the business glossary as Marmot glossary terms, and assign them to the assets they are curated onto |
| include_lineage | bool | false | Import lineage between imported assets |
| include_mlmodels | bool | false | Import machine learning models |
| include_pipelines | bool | false | Import orchestration pipelines |
| include_run_history | bool | false | Import recent pipeline executions as run history |
| include_search_indexes | bool | false | Import search indices |
| include_stored_procedures | bool | false | Import stored procedures as functions |
| include_tables | bool | false | Import databases, tables and views |
| include_tasks | bool | false | Import the individual tasks of each pipeline |
| include_topics | bool | false | Import messaging topics |
| insecure_skip_verify | bool | false | Skip TLS certificate verification |
| jwt_token | string | true | Bot token or personal access token from OpenMetadata |
| link_to_openmetadata | bool | false | Add a link back to the entity in OpenMetadata on every asset |
| naming | select | false | native names assets the way Marmot's own plugin for each technology names them, so a later native run merges with the imported assets. qualified uses the full OpenMetadata path, which keeps two services of the same technology apart |
| page_size | int | false | Entities per API request |
| run_history_days | int | false | How many days of pipeline executions to import |
| run_history_limit | int | false | Maximum executions to import per pipeline |
| service_types | []string | false | Only import these OpenMetadata service types, for example Postgres or Kafka (all if empty) |
| services | []string | false | Only import these OpenMetadata services (all if empty) |
| source_priority | int | false | Priority of OpenMetadata against other sources of the same asset. Lower wins |
| tags | []string | false | Tags to apply to discovered assets |
| tags_from_openmetadata | bool | false | Copy OpenMetadata classification tags onto assets |
| timeout_seconds | int | false | Per-request timeout |

## Available Metadata

The following metadata fields are available:

| Field | Type | Description |
|-------|------|-------------|
| algorithm | string | Algorithm the model uses |
| bucket | string | Top level container the object lives in |
| chart_count | int | Number of charts on the dashboard |
| chart_type | string | Chart type reported by the BI tool |
| cleanup_policies | []string | Topic cleanup policies |
| collection | string | API collection the endpoint belongs to |
| column_count | int | Number of columns |
| concurrency | int | Maximum concurrent runs |
| dashboard_type | string | Dashboard type reported by the BI tool |
| data_model_type | string | Data model type reported by the BI tool |
| data_products | []string | OpenMetadata data products the entity belongs to |
| database | string | Database name |
| domains | []string | OpenMetadata domains the entity belongs to |
| downstream_tasks | []string | Tasks that run after this one |
| endpoint_url | string | URL of the endpoint |
| feature_count | int | Number of features |
| field_count | int | Number of fields in the index |
| file_formats | []string | File formats found in the container |
| glossary_terms | []string | Glossary terms assigned to the entity |
| image_repository | string | Repository holding the model image |
| index_type | string | Index type |
| max_message_size | int | Maximum message size in bytes |
| method | string | HTTP method |
| object_count | int64 | Number of objects |
| object_type | string | OpenMetadata table type, for example Regular, View or MaterializedView |
| openmetadata.fqn | string | Fully qualified name of the entity in OpenMetadata |
| openmetadata.id | string | OpenMetadata entity id |
| openmetadata.service | string | OpenMetadata service the entity belongs to |
| openmetadata.service_type | string | OpenMetadata service type, for example Postgres or Looker |
| openmetadata.updated_at | string | When the entity last changed in OpenMetadata |
| openmetadata.url | string | Address of the entity in the OpenMetadata UI |
| owners | []string | Users or teams that own the entity in OpenMetadata |
| partitioned | bool | Whether the container is partitioned |
| partitions | int | Number of partitions |
| path | string | Request path |
| pipeline | string | Pipeline a task belongs to |
| prefix | string | Path prefix within the bucket |
| primary_key | []string | Columns forming the primary key |
| procedure_type | string | Stored procedure type |
| project | string | Project or workspace the dashboard belongs to |
| replication_factor | int | Replication factor |
| retention_ms | int64 | Retention time in milliseconds |
| retention_size | int64 | Retention size in bytes |
| row_count | int64 | Row count from the OpenMetadata profiler |
| schedule_interval | string | Schedule the pipeline runs on |
| schema | string | Schema name |
| schema_type | string | Message schema type, for example Avro or JSON |
| server | string | Address the model is served from |
| size | int64 | Size in bytes |
| storage | string | Where the model artefact is stored |
| table_name | string | Object name |
| target | string | Column the model predicts |
| task_count | int | Number of tasks in the pipeline |
| task_type | string | Task type, for example the Airflow operator |
| shared | bool | Whether the drive directory or file is shared |
| file_type | string | Drive file type, for example Document or Spreadsheet |
| file_extension | string | Drive file extension |
| mime_type | string | Drive file MIME type |
| directory_type | string | Drive directory type |
| path | string | Path within the drive |
| weekly_query_count | int | Queries against the table in the last week |
