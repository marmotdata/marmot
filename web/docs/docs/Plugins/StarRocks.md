---
title: StarRocks
description: Discovers databases, tables, views and materialized views from StarRocks clusters.
status: experimental
---

# StarRocks

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


The StarRocks plugin discovers databases, tables, views and asynchronous materialized views from StarRocks clusters. StarRocks speaks the MySQL wire protocol, so the plugin connects to a frontend (FE) node on its query port, port 9030 by default.

Tables carry the details that make a StarRocks table different from a plain SQL table: the key model (`DUPLICATE`, `AGGREGATE`, `UNIQUE` or `PRIMARY`), the aggregate function of each value column, the partitioning strategy, the bucketing strategy and the sort key.

## Catalogs

Discovery runs against one catalog per configuration block. `catalog` defaults to `default_catalog`, the cluster's own storage. Naming an external catalog switches the session to it and discovers the tables it exposes. Add a second run to cover a second catalog.

## Lineage

A `Database` asset contains every table and view in it. A view points back at the tables its query reads, resolved against the objects discovered in the same run, and an asynchronous materialized view points back at the base tables StarRocks itself tracks for refreshes. Tables that declare a `foreign_key_constraints` property point at the tables they reference.

## Row Counts

Row counts and sizes come from `information_schema.tables`, which StarRocks fills in from backend tablet reports. A cluster that has not reported yet returns 0 for both. Set `include_statistics: false` to leave them out.

## Example Configuration

```yaml

host: "starrocks-fe.internal"
port: 9030
user: "marmot_reader"
password: "${STARROCKS_PASSWORD}"
catalog: "default_catalog"
databases:
  - "shop"
  - "analytics"
include_columns: true
include_views: true
include_materialized_views: true
include_statistics: true
tags:
  - "starrocks"

```

## Configuration
The following configuration options are available:

| Property | Type | Required | Description |
|----------|------|----------|-------------|
| catalog | string | false | Catalog to discover; an external catalog name switches discovery to it |
| databases | []string | false | Databases to discover; all when empty |
| exclude_databases | []string | false | Databases to skip |
| external_links | []ExternalLink | false | External links to show on all assets |
| filter | Filter | false | Filter discovered assets by name (regex) |
| host | string | true | Frontend (FE) hostname or IP address |
| include_columns | bool | false | Whether to include column information |
| include_materialized_views | bool | false | Whether to discover asynchronous materialized views |
| include_statistics | bool | false | Whether to include row counts, sizes and column counts |
| include_views | bool | false | Whether to discover views |
| password | string | false | Password for authentication |
| port | int | false | Frontend MySQL protocol port |
| tags | TagsConfig | false | Tags to apply to discovered assets |
| tls | string | false | TLS mode (false, true, skip-verify, preferred) |
| user | string | true | Username for authentication |

## Available Metadata

The following metadata fields are available:

| Field | Type | Description |
|-------|------|-------------|
| aggregation_type | string | Aggregate function of a value column in an AGGREGATE KEY table |
| buckets | int | Number of buckets, when fixed |
| catalog | string | Catalog name |
| catalog_type | string | Catalog type (Internal, Hive, Iceberg, ...) |
| column_name | string | Column name |
| comment | string | Table comment |
| created | string | Creation timestamp |
| data_type | string | Column type as printed by SHOW FULL COLUMNS |
| database | string | Database name |
| ddl | string | CREATE statement as printed by SHOW CREATE |
| default_expression | string | Default value |
| description | string | Column comment |
| distribution | string | Bucketing strategy (HASH, RANDOM) |
| distribution_columns | string | Comma-separated hash distribution columns |
| engine | string | Storage engine as reported by information_schema |
| external_engine | string | Engine of an external table (MySQL, Hive, JDBC, ...) |
| host | string | Frontend hostname |
| is_active | bool | Whether the materialized view is active |
| is_auto_increment | bool | Whether the column is AUTO_INCREMENT |
| is_key | bool | Whether StarRocks reports the column as a key column |
| is_nullable | bool | Whether null values are allowed |
| is_primary_key | bool | Whether the column is in the PRIMARY KEY or UNIQUE KEY clause |
| is_sorting_key | bool | Whether the column is in the DUPLICATE KEY, AGGREGATE KEY or ORDER BY clause |
| key_columns | string | Comma-separated key columns |
| key_model | string | Table key model (DUPLICATE, AGGREGATE, UNIQUE, PRIMARY) |
| last_refresh_start_time | string | Start time of the last materialized view refresh |
| last_refresh_state | string | State of the last materialized view refresh |
| materialized | bool | Whether a view is an asynchronous materialized view |
| object_type | string | Object type (table, view, materialized_view, external_table) |
| order_by | string | Comma-separated sort key columns from ORDER BY |
| partition_columns | string | Comma-separated partition columns |
| partition_count | int | Number of partitions |
| partition_expression | string | Partition expression for expression partitioning |
| partition_type | string | Partitioning strategy (RANGE, LIST, EXPRESSION) |
| port | int | Frontend MySQL protocol port |
| refresh_type | string | Materialized view refresh type (ASYNC, MANUAL) |
| replication_num | int | Replica count from table properties |
| starrocks_version | string | StarRocks version reported by the frontend |
| storage_volume | string | Storage volume from table properties |
| table_count | int | Number of tables discovered in the database |
| table_name | string | Bare table or view name |
| task_name | string | Refresh task name of the materialized view |
| updated | string | Last data change timestamp |
| view_count | int | Number of views and materialized views discovered in the database |
