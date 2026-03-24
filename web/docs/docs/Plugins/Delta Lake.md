---
title: Delta Lake
description: This plugin discovers tables from Delta Lake transaction logs on local filesystems.
status: experimental
---

# Delta Lake

<div class="flex flex-col gap-3 mb-6 pb-6 border-b border-gray-200">
<div class="flex items-center gap-3">
<span class="inline-flex items-center rounded-full px-4 py-2 text-sm font-medium bg-earthy-yellow-300 text-earthy-yellow-900">Experimental</span>
</div>
<div class="flex items-center gap-2">
<span class="text-sm text-gray-500">Creates:</span>
<div class="flex flex-wrap gap-2"><span class="inline-flex items-center rounded-lg px-4 py-2 text-sm font-medium bg-earthy-green-100 text-earthy-green-800 border border-earthy-green-300">Assets</span></div>
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



## Example Configuration

```yaml

table_paths:
  - "/data/delta/events"
  - "/data/delta/users"
tags:
  - "delta-lake"

```

## Configuration
The following configuration options are available:

| Property | Type | Required | Description |
|----------|------|----------|-------------|
| external_links | []ExternalLink | false | External links to show on all assets |
| filter | Filter | false | Filter discovered assets by name (regex) |
| table_paths | []string | false | Paths to Delta Lake table directories |
| tags | TagsConfig | false | Tags to apply to discovered assets |

## Available Metadata

The following metadata fields are available:

| Field | Type | Description |
|-------|------|-------------|
| created_time | int64 | Table creation timestamp in milliseconds |
| current_version | int64 | Current Delta log version |
| format | string | Data format (e.g. parquet) |
| location | string | Table directory path |
| min_reader_version | int | Minimum reader protocol version |
| min_writer_version | int | Minimum writer protocol version |
| num_files | int | Number of active data files |
| partition_columns | string | Comma-separated partition column names |
| schema_field_count | int | Number of schema fields |
| table_id | string | Delta table unique identifier |
| total_size | int64 | Total size of active data files in bytes |