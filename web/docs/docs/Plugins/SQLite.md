---
title: SQLite
description: Discovers tables, views and foreign key relationships from SQLite database files.
status: experimental
---

# SQLite

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


The SQLite plugin discovers tables, views and foreign key relationships from SQLite database files. It opens the file read-only with the pure-Go `modernc.org/sqlite` driver, so it needs no cgo. Turso and libSQL databases use the SQLite on-disk format, so a local copy of one is discovered the same way as any other SQLite file.

## File Sources

The `path` field accepts local paths, S3 URIs (`s3://bucket/key`) or Git URIs (`git::https://...`). For S3 and Git sources, the file is downloaded to a temporary directory before discovery and cleaned up afterwards.

See [File Sources](./Shared%20Configuration/File%20Sources.md) for the full list of supported backends, authentication options and configuration examples.



## Example Configuration

```yaml

path: "/data/app.db"
include_columns: true
enable_metrics: true
discover_foreign_keys: true
exclude_system_tables: true
filter:
  include:
    - "^user.*"
  exclude:
    - ".*_tmp$"
tags:
  - "sqlite"
  - "app"

```

## Configuration
The following configuration options are available:

| Property | Type | Required | Description |
|----------|------|----------|-------------|
| discover_foreign_keys | bool | false | Whether to discover foreign key relationships |
| enable_metrics | bool | false | Whether to include table metrics (row and column counts) |
| exclude_system_tables | bool | false | Whether to exclude SQLite internal tables (sqlite_*) |
| external_links | []ExternalLink | false | External links to show on all assets |
| filter | Filter | false | Filter discovered assets by name (regex) |
| git_source | GitSourceConfig | false | Git repository file source configuration |
| include_columns | bool | false | Whether to include column information in table metadata |
| path | string | false | Path to the SQLite database file (local path, s3://bucket/key or git::url) |
| s3_source | S3SourceConfig | false | S3 file source configuration |
| source_type | string | false | File source backend (auto-detected from path when empty) |
| tags | TagsConfig | false | Tags to apply to discovered assets |

## Available Metadata

The following metadata fields are available:

| Field | Type | Description |
|-------|------|-------------|
| column_default | string | Default value expression |
| column_name | string | Column name |
| data_type | string | Declared column data type |
| is_nullable | bool | Whether null values are allowed |
| is_primary_key | bool | Whether the column is part of the primary key |
| object_type | string | Object type (table, view) |
| path | string | Path to the SQLite database file |
| source_column | string | Column in the referencing table |
| source_table | string | Name of the referencing table |
| table_name | string | Table or view name |
| target_column | string | Column in the referenced table |
| target_table | string | Name of the referenced table |
