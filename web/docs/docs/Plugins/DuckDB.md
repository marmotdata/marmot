---
title: DuckDB
description: Discovers schemas, tables, views and foreign key relationships from DuckDB database files.
status: experimental
---

# DuckDB

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
  href="/docs/Populating/UI"
  buttonText="View Guide"
  variant="secondary"
  icon="mdi:cursor-default-click"
/>


The DuckDB plugin discovers schemas, tables, views and foreign key relationships from DuckDB database files. It extracts column details and table metrics like estimated sizes using DuckDB's built-in metadata functions.

## Connection Examples

import { Collapsible } from "@site/src/components/Collapsible";

<Collapsible title="Local Database File" icon="devicon:duckdb">

```yaml
path: "/data/analytics.duckdb"
include_columns: true
enable_metrics: true
discover_foreign_keys: true
tags:
  - "duckdb"
  - "analytics"
```

</Collapsible>

<Collapsible title="With Filters" icon="mdi:filter">

```yaml
path: "/data/analytics.duckdb"
include_columns: true
enable_metrics: true
filter:
  include:
    - "^main\\..*"
  exclude:
    - ".*_temp$"
tags:
  - "duckdb"
```

</Collapsible>



## Example Configuration

```yaml

path: "/data/analytics.duckdb"
include_columns: true
enable_metrics: true
discover_foreign_keys: true
exclude_system_schemas: true
filter:
  include:
    - "^main\\..*"
  exclude:
    - ".*_temp$"
tags:
  - "duckdb"
  - "analytics"

```

## Configuration
The following configuration options are available:

| Property | Type | Required | Description |
|----------|------|----------|-------------|
| discover_foreign_keys | bool | false | Whether to discover foreign key relationships |
| enable_metrics | bool | false | Whether to include table metrics (row counts and sizes) |
| exclude_system_schemas | bool | false | Whether to exclude system schemas (information_schema, pg_catalog) |
| external_links | []ExternalLink | false | External links to show on all assets |
| filter | Filter | false | Filter discovered assets by name (regex) |
| include_columns | bool | false | Whether to include column information in table metadata |
| path | string | false | Path to the DuckDB database file |
| tags | TagsConfig | false | Tags to apply to discovered assets |

## Available Metadata

The following metadata fields are available:

| Field | Type | Description |
|-------|------|-------------|
| column_default | string | Default value expression |
| column_name | string | Column name |
| comment | string | Object comment/description |
| constraint_name | string | Foreign key constraint name |
| data_type | string | Column data type |
| is_nullable | bool | Whether null values are allowed |
| object_type | string | Object type (BASE TABLE, VIEW) |
| path | string | Path to the DuckDB database file |
| row_count | int64 | Estimated row count |
| schema | string | Schema name |
| size | int64 | Estimated size in bytes |
| source_column | string | Column in the referencing table |
| source_schema | string | Schema of the referencing table |
| source_table | string | Name of the referencing table |
| table_name | string | Table or view name |
| target_column | string | Column in the referenced table |
| target_schema | string | Schema of the referenced table |
| target_table | string | Name of the referenced table |