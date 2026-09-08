---
title: Unity Catalog
description: Discovers catalogs, tables, views, volumes, functions and registered models from Unity Catalog servers.
status: experimental
---

# Unity Catalog

<div class="flex flex-col gap-3 mb-6 pb-6 border-b border-gray-200">
<div class="flex items-center gap-3">
<span class="inline-flex items-center rounded-full px-4 py-2 text-sm font-medium bg-earthy-yellow-300 text-earthy-yellow-900">Experimental</span>
</div>
<div class="flex items-center gap-2">
<span class="text-sm text-gray-500">Creates:</span>
<div class="flex flex-wrap gap-2"><span class="inline-flex items-center rounded-lg px-4 py-2 text-sm font-medium bg-earthy-green-100 text-earthy-green-800 border border-earthy-green-300">Assets</span><span class="inline-flex items-center rounded-lg px-4 py-2 text-sm font-medium bg-earthy-green-100 text-earthy-green-800 border border-earthy-green-300">Lineage</span></div>
</div>
</div>

import { CalloutCard } from '@site/src/components/DocCard';

<CalloutCard
  title="Configure in the UI"
  description="This plugin can be configured directly in the Marmot UI with a step-by-step wizard."
  docId="Populating/UI"
  buttonText="View Guide"
  variant="secondary"
  icon="mdi:cursor-default-click"
/>


The Unity Catalog plugin discovers catalogs, tables, views, volumes, functions and registered models from a Unity Catalog server. It reads the open-source Unity Catalog REST API at `{host}/api/2.1/unity-catalog`, so it works against a self-hosted server and against anything else serving the same API.

Schemas are not assets of their own. Every object is named by its three-part `catalog.schema.object` name and is linked to its catalog, so the same table name in two schemas stays two assets.

## Authentication

`token` is sent as a bearer token when it is set. A server running without authentication needs no token.

## Lineage

- Each catalog CONTAINS every table, view, volume, function and model under it.
- A view gets a VIEW_OF edge from each base table its definition reads. One- and two-part references resolve against the view's own catalog and schema.
- A table with foreign key constraints gets a FOREIGN_KEY edge to each parent table found in the same run. The open-source server does not store constraints, so these come from Databricks-compatible servers only.
- A table or volume on cloud object storage gets a FEEDS edge from its bucket or container: `s3://` from the S3 plugin, `gs://` from GCS and `abfss://` from Azure Blob Storage. The bucket asset belongs to those plugins; this plugin only emits the edge.

## Example Configuration

```yaml

host: "http://localhost:8080"
token: "dapi-your-token"
catalogs:
  - "unity"
  - "shop"
exclude_catalogs:
  - "system"
  - "__databricks_internal"
include_columns: true
include_volumes: true
include_functions: true
include_models: true
tags:
  - "unity-catalog"

```

## Configuration
The following configuration options are available:

| Property | Type | Required | Description |
|----------|------|----------|-------------|
| catalogs | []string | false | Catalogs to discover (all when empty) |
| exclude_catalogs | []string | false | Catalogs to skip |
| external_links | []ExternalLink | false | External links to show on all assets |
| filter | Filter | false | Filter discovered assets by name (regex) |
| host | string | true | Unity Catalog server URL, for example http://localhost:8080 |
| include_columns | bool | false | Whether to include column information in table metadata |
| include_functions | bool | false | Whether to discover functions |
| include_models | bool | false | Whether to discover registered models |
| include_volumes | bool | false | Whether to discover volumes |
| page_size | int | false | Objects requested per API page |
| tags | TagsConfig | false | Tags to apply to discovered assets |
| token | string | false | Bearer token, when the server requires one |
| verify_ssl | bool | false | Whether to verify the server's TLS certificate |

## Available Metadata

The following metadata fields are available:

| Field | Type | Description |
|-------|------|-------------|
| catalog | string | Catalog the object belongs to |
| catalog_id | string | Catalog id |
| catalog_name | string | Catalog name |
| column_name | string | Column name |
| comment | string | Object comment |
| created_at | string | Creation time (RFC 3339) |
| data_source_format | string | Data source format (DELTA, PARQUET, CSV, ...) |
| data_type | string | Column type as written (type_text) |
| description | string | Column comment |
| foreign_keys | []object | Foreign key constraints (columns, parent_table, parent_columns) |
| function_id | string | Function id |
| function_name | string | Function name |
| is_deterministic | bool | Whether the function is deterministic |
| is_nullable | bool | Whether null values are allowed |
| is_primary_key | bool | Whether the column is part of the primary key |
| language | string | Language the function body is written in (SQL, python, ...) |
| latest_version | int | Highest registered model version number |
| model_id | string | Model id |
| model_name | string | Model name |
| owner | string | Object owner |
| parameters | []string | Function input parameters as "name type", in declaration order |
| partition_columns | []string | Partition columns in partition order |
| partition_index | int | Position in the partition key, for partition columns |
| primary_key | []string | Primary key columns |
| properties | map[string]string | User-defined properties |
| return_type | string | Function return type |
| routine_body | string | Routine body kind (SQL, EXTERNAL) |
| schema | string | Schema the object belongs to |
| schema_count | int | Number of schemas in the catalog |
| sql_data_access | string | SQL data access (CONTAINS_SQL, READS_SQL_DATA, NO_SQL) |
| storage_location | string | Storage location of the data |
| table_count | int | Number of tables and views in the catalog |
| table_id | string | Table id |
| table_name | string | Table or view name |
| table_type | string | Table type (MANAGED, EXTERNAL, VIEW, MATERIALIZED_VIEW, STREAMING_TABLE, FOREIGN) |
| type_name | string | Column type name (INT, STRING, DECIMAL, ...) |
| updated_at | string | Last update time (RFC 3339) |
| version_count | int | Number of registered model versions |
| versions | map[string]string | Model version number to status (READY, PENDING_REGISTRATION, ...) |
| volume_id | string | Volume id |
| volume_name | string | Volume name |
| volume_type | string | Volume type (MANAGED, EXTERNAL) |
