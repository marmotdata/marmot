---
title: ClickHouse
description: Discovers databases, tables, and views from ClickHouse instances.
status: experimental
---

# ClickHouse

<div class="flex flex-col gap-3 mb-6 pb-6 border-b border-gray-200">
<div class="flex items-center gap-3">
<span class="inline-flex items-center rounded-full px-4 py-2 text-sm font-medium bg-earthy-yellow-300 text-earthy-yellow-900">Experimental</span>
</div>
<div class="flex items-center gap-2">
<span class="text-sm text-gray-500">Creates:</span>
<div class="flex flex-wrap gap-2"><span class="inline-flex items-center rounded-lg px-4 py-2 text-sm font-medium bg-earthy-green-100 text-earthy-green-800 border border-earthy-green-300">Assets</span></div>
</div>
</div>

The ClickHouse plugin discovers metadata from ClickHouse databases, including databases, tables, views, and materialized views. It extracts schema information, column details, and table metrics like row counts and storage sizes.

## Prerequisites

- ClickHouse server accessible via the native protocol (default port 9000)
- User with read access to `system.databases`, `system.tables`, and `system.columns`

:::tip[Connection]
The plugin uses the ClickHouse native protocol (port 9000 by default), not the HTTP interface (port 8123). For secure connections, enable the `secure` option.
:::

## Example Configuration

```yaml
host: "clickhouse.company.com"
port: 9000
user: "default"
password: "${CLICKHOUSE_PASSWORD}"
database: "default"
secure: false
include_databases: true
include_columns: true
enable_metrics: true
exclude_system_tables: true
database_filter:
  include:
    - "^analytics.*"
  exclude:
    - ".*_temp$"
tags:
  - "clickhouse"
  - "analytics"
```

## Configuration

The following configuration options are available:

| Property | Type | Required | Description |
|----------|------|----------|-------------|
| host | string | true | ClickHouse server hostname or IP address |
| port | int | false | ClickHouse native protocol port (default: 9000) |
| user | string | true | Username for authentication |
| password | string | false | Password for authentication |
| database | string | false | Default database to connect to (default: "default") |
| secure | bool | false | Use TLS/SSL connection (default: false) |
| include_databases | bool | false | Whether to discover databases (default: true) |
| include_columns | bool | false | Whether to include column information in table metadata (default: true) |
| enable_metrics | bool | false | Whether to include table metrics like row counts (default: true) |
| exclude_system_tables | bool | false | Whether to exclude system tables (default: true) |
| database_filter | Filter | false | Filter configuration for databases (include/exclude regex) |
| table_filter | Filter | false | Filter configuration for tables (include/exclude regex) |
| tags | []string | false | Tags to apply to discovered assets |
| external_links | []ExternalLink | false | External links to show on all assets |

## Available Metadata

The following metadata fields are available:

### Database Metadata

| Field | Type | Description |
|-------|------|-------------|
| database | string | Database name |
| engine | string | Database engine type |
| comment | string | Database comment/description |

### Table Metadata

| Field | Type | Description |
|-------|------|-------------|
| database | string | Parent database name |
| table_name | string | Table name |
| engine | string | Table engine (MergeTree, ReplacingMergeTree, etc.) |
| row_count | int64 | Estimated row count |
| size_bytes | int64 | Table size in bytes |
| comment | string | Table comment/description |
| columns | array | Column information (if include_columns is enabled) |

### Column Metadata

| Field | Type | Description |
|-------|------|-------------|
| column_name | string | Column name |
| data_type | string | Column data type |
| is_primary_key | bool | Whether column is part of primary key |
| is_sorting_key | bool | Whether column is part of sorting key |
| default_kind | string | Default value kind (DEFAULT, MATERIALIZED, ALIAS) |
| default_expression | string | Default value expression |
| comment | string | Column comment/description |
