---
title: Doris
description: Discovers databases, tables, views and materialized views from Apache Doris clusters.
status: experimental
---

# Doris

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


The Doris plugin discovers databases, tables, views and async materialized views from Apache Doris clusters. It connects to a frontend over the MySQL protocol and reads the table model (DUPLICATE, UNIQUE, AGGREGATE), partitioning, distribution and column aggregation types from the DDL. Tables and views are named `<database>.<table>`, so the same table name in two databases stays apart.

Views and materialized views carry their defining query, with `VIEW_OF` lineage from the tables they read. Foreign keys declared with `ALTER TABLE ... ADD CONSTRAINT` become `FOREIGN_KEY` lineage. Row counts and data sizes come from `information_schema.TABLES`, which Doris fills from the backends' periodic tablet report, so a freshly loaded table can show zero rows for a minute or two.

## Required Permissions

```sql
CREATE USER 'marmot_reader'@'%' IDENTIFIED BY 'your-password';
GRANT SELECT_PRIV, SHOW_VIEW_PRIV ON internal.*.* TO 'marmot_reader'@'%';
```

`SHOW_VIEW_PRIV` lets `SHOW CREATE VIEW` run; without it views are catalogued without their query or lineage.



## Example Configuration

```yaml

host: "doris-fe.internal"
port: 9030
user: "marmot_reader"
password: "${DORIS_PASSWORD}"
databases:
  - "shop"
  - "analytics"
tags:
  - "doris"
  - "warehouse"

```

## Configuration
The following configuration options are available:

| Property | Type | Required | Description |
|----------|------|----------|-------------|
| catalog | string | false | Doris catalog to discover (default `internal`) |
| databases | []string | false | Databases to discover (every database except exclude_databases when empty) |
| exclude_databases | []string | false | Databases to skip when databases is empty (default `information_schema`, `mysql`, `__internal_schema`) |
| external_links | []ExternalLink | false | External links to show on all assets |
| filter | Filter | false | Filter discovered assets by name (regex) |
| host | string | true | Doris frontend hostname or IP address |
| include_columns | bool | false | Whether to include column information in table metadata |
| include_materialized_views | bool | false | Whether to discover async materialized views |
| include_statistics | bool | false | Whether to include row counts, data sizes and column counts |
| include_views | bool | false | Whether to discover views |
| password | string | false | Password for authentication |
| port | int | false | Doris frontend MySQL protocol port (default 9030) |
| tags | TagsConfig | false | Tags to apply to discovered assets |
| tls | string | false | TLS configuration (false, true, skip-verify, preferred) |
| user | string | true | Username for authentication |

## Available Metadata

The following metadata fields are available:

| Field | Type | Description |
|-------|------|-------------|
| aggregation_type | string | Aggregation of a value column on an AGGREGATE table (SUM, MAX, REPLACE, HLL_UNION, BITMAP_UNION) |
| auto_bucket | bool | Whether Doris picks the bucket count |
| buckets | int | Number of buckets per partition |
| catalog | string | Doris catalog the object lives in |
| column_name | string | Column name |
| comment | string | Table or view comment |
| created | string | Creation timestamp |
| data_type | string | Declared type as Doris prints it (decimalv3(9, 2), array\<int>) |
| database | string | Database name |
| ddl | string | SHOW CREATE output |
| default_expression | string | Default value |
| description | string | Column comment |
| distribution_columns | []string | Columns the data is hashed on |
| distribution_type | string | Bucketing (HASH, RANDOM) |
| doris_version | string | Doris build the frontend reports |
| engine | string | Storage engine (Doris, View, MATERIALIZED_VIEW, or the external system) |
| host | string | Doris frontend hostname |
| is_key | bool | Whether the column is part of the table key |
| is_nullable | bool | Whether null values are allowed |
| is_primary_key | bool | Key column of a UNIQUE or AGGREGATE table |
| is_sorting_key | bool | Key column of a DUPLICATE table |
| job_name | string | Refresh job of a materialized view |
| key_columns | []string | Columns that make up the key |
| key_model | string | Table model (DUPLICATE, UNIQUE, AGGREGATE, none) |
| materialized | bool | Whether the view is an async materialized view |
| mv_partition_info | string | How a materialized view is partitioned |
| object_type | string | Object type (table, view, materialized_view, external_table) |
| partition_columns | []string | Columns or expressions the table is partitioned by |
| partition_count | int | Number of partitions |
| partition_type | string | Partitioning (RANGE, LIST, none) |
| port | int | Doris frontend MySQL protocol port |
| refresh_info | string | Build and refresh settings of a materialized view |
| refresh_state | string | Outcome of the last materialized view refresh |
| replication | string | Replication allocation or replica count |
| state | string | State of a materialized view (NORMAL, SCHEMA_CHANGE) |
| storage_medium | string | Storage medium (hdd, ssd) |
| table_count | int | Tables in the database |
| table_name | string | Object name without the database |
| updated | string | Last update timestamp |
| view_count | int | Views and materialized views in the database |
