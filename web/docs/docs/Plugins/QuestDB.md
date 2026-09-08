---
title: QuestDB
description: Discovers tables, views and materialized views from QuestDB instances.
status: experimental
---

# QuestDB

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


The QuestDB plugin discovers tables, views and materialized views from QuestDB instances over the PostgreSQL wire protocol (port 8812). It reads QuestDB's own table functions (`tables()`, `table_columns()`, `table_partitions()`, `views()`, `materialized_views()`) rather than `pg_catalog`, so it captures the designated timestamp, partitioning, WAL, deduplication and TTL settings of each table, the QuestDB column types verbatim (`SYMBOL`, `GEOHASH(8c)`, `IPv4`, `LONG256`), row counts, sizes on disk and partition counts.

Every view carries its defining SQL as the asset query. A materialized view is linked to its base table, and a plain view to every table its SQL reads, with `VIEW_OF` lineage.

QuestDB has a single database and no schemas, so no Database asset is created: tables and views are addressed by their bare name, the same identity an OpenMetadata import of a QuestDB service produces.

## Required Permissions

QuestDB open source has a single login, `admin` with password `quest` by default, and no per-table permissions. On QuestDB Enterprise, use a user that can SELECT from the tables to discover; see QuestDB's access control documentation for the exact grants.



## Example Configuration

```yaml

host: "questdb.internal"
port: 8812
user: "admin"
password: "quest"
include_statistics: true
tags:
  - "questdb"
  - "timeseries"

```

## Configuration
The following configuration options are available:

| Property | Type | Required | Description |
|----------|------|----------|-------------|
| database | string | false | Database name, QuestDB has only one |
| exclude_system_tables | bool | false | Whether to exclude QuestDB internal tables (sys.*, _*, telemetry) |
| external_links | []ExternalLink | false | External links to show on all assets |
| filter | Filter | false | Filter discovered assets by name (regex) |
| host | string | true | QuestDB server hostname or IP address |
| include_columns | bool | false | Whether to include column information in table metadata |
| include_materialized_views | bool | false | Whether to discover materialized views |
| include_statistics | bool | false | Whether to include row counts, column counts and sizes |
| include_views | bool | false | Whether to discover views |
| password | string | false | Password for authentication (QuestDB ships with quest) |
| port | int | false | PostgreSQL wire protocol port |
| ssl_mode | string | false | SSL mode (disable, require) |
| tags | TagsConfig | false | Tags to apply to discovered assets |
| user | string | false | Username for authentication |

## Available Metadata

The following metadata fields are available:

| Field | Type | Description |
|-------|------|-------------|
| base_table | string | Table a materialized view is built from |
| column_name | string | Column name |
| data_type | string | QuestDB data type as reported, for example SYMBOL or GEOHASH(8c) |
| dedup | bool | Whether deduplication on upsert keys is enabled |
| designated_timestamp | string | Column the table is ordered and partitioned by |
| designated_timestamp | bool | Whether this is the designated timestamp column |
| host | string | QuestDB server hostname |
| indexed | bool | Whether the symbol column has a bitmap index |
| is_nullable | bool | Always true, QuestDB columns are nullable |
| last_refresh | string | When the materialized view last started refreshing |
| materialized | bool | Whether a view is materialized |
| max_uncommitted_rows | int64 | Rows buffered before an out-of-order commit |
| o3_max_lag | int64 | Out-of-order commit lag in microseconds |
| object_type | string | Object type (table, view, materialized_view) |
| partition_by | string | Partition interval (NONE, HOUR, DAY, WEEK, MONTH, YEAR) |
| partition_count | int64 | Number of partitions on disk |
| port | int | PostgreSQL wire protocol port |
| refresh_period | string | Materialized view refresh interval, for example 1 HOUR |
| refresh_type | string | Materialized view refresh type (immediate, timer, manual, period) |
| symbol_cached | bool | Whether the symbol table is cached in memory |
| symbol_capacity | int64 | Distinct values a symbol column is sized for |
| table_name | string | Table or view name |
| ttl | string | Retention period, for example 1 WEEK |
| upsert_key | bool | Whether the column is a deduplication upsert key |
| wal_enabled | bool | Whether writes go through the write-ahead log |
