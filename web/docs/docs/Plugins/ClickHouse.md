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


The ClickHouse plugin discovers databases, tables, and views from ClickHouse instances. It extracts schema information, column details, and table metrics like row counts and storage sizes.

## Connection Examples

import { Collapsible } from "@site/src/components/Collapsible";

<Collapsible title="Basic Connection" icon="simple-icons:clickhouse">

```yaml
host: "clickhouse.company.com"
port: 9000
user: "default"
password: "${CLICKHOUSE_PASSWORD}"
database: "default"
include_databases: true
include_columns: true
enable_metrics: true
tags:
  - "clickhouse"
  - "analytics"
```

</Collapsible>

<Collapsible title="ClickHouse Cloud" icon="mdi:cloud">

```yaml
host: "your-instance.clickhouse.cloud"
port: 9440
user: "default"
password: "${CLICKHOUSE_PASSWORD}"
secure: true
include_databases: true
include_columns: true
enable_metrics: true
database_filter:
  include:
    - "^analytics.*"
  exclude:
    - ".*_temp$"
tags:
  - "clickhouse"
  - "cloud"
```

</Collapsible>

## Required Permissions

The user needs read access to system tables:

```sql
GRANT SELECT ON system.databases TO marmot_user;
GRANT SELECT ON system.tables TO marmot_user;
GRANT SELECT ON system.columns TO marmot_user;
```

For read-only discovery of all databases:

```sql
GRANT SHOW DATABASES ON *.* TO marmot_user;
GRANT SHOW TABLES ON *.* TO marmot_user;
GRANT SHOW COLUMNS ON *.* TO marmot_user;
```


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
| database | string | false | Default database to connect to |
| database_filter | plugin.Filter | false | Filter configuration for databases |
| enable_metrics | bool | false | Whether to include table metrics (row counts, sizes) |
| exclude_system_tables | bool | false | Whether to exclude system tables |
| external_links | []ExternalLink | false | External links to show on all assets |
| host | string | false | ClickHouse server hostname or IP address |
| include_columns | bool | false | Whether to include column information in table metadata |
| include_databases | bool | false | Whether to discover databases |
| password | string | false | Password for authentication |
| port | int | false | ClickHouse native protocol port |
| secure | bool | false | Use TLS/SSL connection |
| table_filter | plugin.Filter | false | Filter configuration for tables |
| tags | TagsConfig | false | Tags to apply to discovered assets |
| user | string | false | Username for authentication |

## Available Metadata

The following metadata fields are available:

| Field | Type | Description |
|-------|------|-------------|
| column_name | string | Column name |
| comment | string | Database comment/description |
| comment | string | Column comment/description |
| comment | string | Table comment/description |
| data_type | string | Column data type |
| database | string | Database name |
| database | string | Parent database name |
| default_expression | string | Default value expression |
| default_kind | string | Default value kind (DEFAULT, MATERIALIZED, ALIAS) |
| engine | string | Database engine type |
| engine | string | Table engine (MergeTree, ReplacingMergeTree, etc.) |
| is_primary_key | bool | Whether column is part of primary key |
| is_sorting_key | bool | Whether column is part of sorting key |
| row_count | int64 | Estimated row count |
| size_bytes | int64 | Table size in bytes |
| table_name | string | Table name |