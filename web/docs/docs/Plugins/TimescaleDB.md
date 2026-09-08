---
title: TimescaleDB
description: Discovers databases, tables, hypertables and continuous aggregates from TimescaleDB instances.
status: experimental
---

# TimescaleDB

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


The TimescaleDB plugin discovers databases, tables, views, hypertables and continuous aggregates. Alongside the usual PostgreSQL metadata it records what makes a table a hypertable: the column it is partitioned by time on, the chunk interval, any space partitions, the chunk count, the compression settings and the background policies that compress, retain and refresh the data.

## Relationship to the PostgreSQL plugin

TimescaleDB is a PostgreSQL extension, so a TimescaleDB server is a PostgreSQL server. Assets are filed under the `PostgreSQL` provider by their bare object name, which is exactly the identity the [PostgreSQL](./PostgreSQL.md) plugin produces. Running both plugins against the same instance updates one set of assets rather than creating two. This plugin is the PostgreSQL plugin plus hypertable knowledge, so there is no reason to run both.

Objects in the `_timescaledb_internal`, `_timescaledb_catalog`, `_timescaledb_config`, `_timescaledb_cache`, `timescaledb_information` and `timescaledb_experimental` schemas are skipped. They hold chunk tables, materialization hypertables and catalog views, and one busy hypertable puts thousands of chunks there.

When a database does not have the extension installed, it is discovered as plain PostgreSQL.

## Hypertables and continuous aggregates

A hypertable is discovered as a `Table` with `hypertable: true` and its partitioning, compression and policy metadata. Its row count and size come from `approximate_row_count` and `hypertable_size` rather than from PostgreSQL's own estimates, which report near zero because a hypertable's parent table holds no rows of its own.

A continuous aggregate is discovered as a `View` with `continuous_aggregate: true`, and gets a `VIEW_OF` edge from the hypertable it reads. PostgreSQL rewrites a continuous aggregate to read from a hidden materialization hypertable, so the query shown is the one recorded in the TimescaleDB catalog, which is the query as it was written.

## Example Configuration

```yaml

host: "timescale.company.com"
port: 5432
user: "marmot_reader"
password: "secure_password_123"
database: "metrics"
ssl_mode: "require"
include_chunks: true
tags:
  - "timescale"
  - "production"

```

## Configuration
The following configuration options are available:

| Property | Type | Required | Description |
|----------|------|----------|-------------|
| database | string | false | Database to discover. When empty, every database on the server is discovered |
| discover_foreign_keys | bool | false | Whether to discover foreign key relationships |
| exclude_databases | []string | false | Databases to skip |
| exclude_system_schemas | bool | false | Whether to exclude PostgreSQL and TimescaleDB internal schemas |
| external_links | []ExternalLink | false | External links to show on all assets |
| filter | Filter | false | Filter discovered assets by name (regex) |
| host | string | true | TimescaleDB server hostname or IP address |
| include_chunks | bool | false | Whether to read the oldest and newest chunk range. Off by default because a busy hypertable has thousands of chunks |
| include_columns | bool | false | Whether to include column information in table metadata |
| include_compression | bool | false | Whether to record compression segment and order columns |
| include_continuous_aggregates | bool | false | Whether to record continuous aggregates and their source hypertable lineage |
| include_hypertables | bool | false | Whether to record hypertable dimensions, chunk counts and policies |
| include_statistics | bool | false | Whether to collect row, column and size statistics |
| password | string | false | Password for authentication |
| port | int | false | TimescaleDB server port |
| ssl_mode | string | false | SSL mode (disable, require, verify-ca, verify-full) |
| tags | TagsConfig | false | Tags to apply to discovered assets |
| user | string | true | Username for authentication |

## Available Metadata

The following metadata fields are available:

| Field | Type | Description |
|-------|------|-------------|
| chunk_range | object | Oldest and newest chunk boundary, when include_chunks is on |
| collate | string | Database collation |
| column_name | string | Column name |
| comment | string | Object comment |
| compression_enabled | bool | Whether compression is enabled |
| compression_order_by | []string | Columns rows are ordered by inside a compressed batch |
| compression_segment_by | []string | Columns compressed batches are grouped by |
| connection_limit | int | Maximum allowed connections to the database |
| continuous_aggregate | bool | Whether the view is a continuous aggregate |
| ctype | string | Database character classification |
| data_type | string | Data type |
| database | string | Database name |
| default_expression | string | Default value expression |
| description | string | Column comment |
| encoding | string | Database encoding |
| finalized | bool | Whether the aggregate uses the finalized form, on TimescaleDB versions that report it |
| host | string | TimescaleDB server hostname |
| hypertable | bool | Whether the table is a hypertable |
| identity | string | Identity kind: a for always, d for by default |
| integer_interval | int64 | Chunk interval when the hypertable is partitioned on an integer column |
| integer_now_func | string | Function that returns the current value of an integer time column |
| is_distributed | bool | Whether the hypertable is distributed, on TimescaleDB versions that support it |
| is_generated | bool | Whether the column is generated from other columns |
| is_nullable | bool | Whether null values are allowed |
| is_primary_key | bool | Whether the column is part of the primary key |
| materialization_hypertable | string | Internal hypertable the results are stored in |
| materialized_only | bool | Whether the view reads only materialized data, without the most recent rows |
| num_chunks | int64 | Number of chunks the hypertable is split into |
| num_dimensions | int | Number of partitioning dimensions |
| object_type | string | Object type (table, view, materialized_view) |
| owner | string | Object owner |
| policies | []object | Background policies attached to the object (compression, retention, refresh) |
| port | int | TimescaleDB server port |
| refresh_policy | object | Policy that refreshes the aggregate |
| schema | string | Schema name |
| size | int64 | Database size in bytes |
| source_hypertable | string | Hypertable the aggregate reads from |
| space_partitions | []object | Partitioning columns beyond time, with their partition counts |
| table_name | string | Table or view name |
| tablespaces | []string | Tablespaces the hypertable's chunks are placed in |
| time_column | string | Column the hypertable is partitioned by time on |
| time_column_type | string | Data type of the time column |
| time_interval | string | Time span covered by one chunk |
| time_interval_seconds | int64 | Time span covered by one chunk, in seconds |
| timescaledb_version | string | Version of the timescaledb extension |
