---
title: Bigtable
description: Discovers instances, tables and column families from Google Cloud Bigtable.
status: experimental
---

# Bigtable

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


The Bigtable plugin discovers the instances in a Google Cloud project, the tables in each instance, and the column families those tables define. Every instance becomes an Instance asset holding its tables through CONTAINS lineage. A table is named `<instance>.<table>`, because table names are only unique within an instance.

## Columns

Bigtable declares column families but not the qualifiers stored under them, and every value is raw bytes. The plugin therefore reads a sample of rows per table (`sample_rows`, 100 by default) and records the `family:qualifier` pairs it saw, how many of the sampled rows held each one, and what the values looked like: `text`, `int64` for the 8 byte number the increment operation writes, or `binary`. A qualifier keeps a specific type only while every sampled value agreed. Each table also gets a synthetic `row_key` column marked as the primary key.

Set `include_columns: false` to skip the sample. Tables then carry only the `row_key` column.

## Row counts

Bigtable keeps no row count, so `include_statistics` has to scan. The scan gives up at `max_count_rows` and reports nothing for that table rather than a number that is really a floor.

## Credentials

With no `credentials` block the plugin uses the credentials the environment already provides: Workload Identity, a service account attached to the machine, or `GOOGLE_APPLICATION_CREDENTIALS`. Supply `credentials.credentials_file` or `credentials.credentials_json` to use a specific service account key.

## Emulator

`emulator_host` points the plugin at a local Bigtable emulator and connects without TLS or credentials. The emulator cannot list its own instances, so `instances` has to be listed when `emulator_host` is set. Tables discovered from an emulator get no Google Cloud Console link.

## Example Configuration

```yaml

project_id: "company-analytics"
instances:
  - "prod-metrics"
credentials:
  credentials_file: "/etc/marmot/bigtable-service-account.json"
include_columns: true
sample_rows: 100
include_statistics: true
max_count_rows: 100000
filter:
  include:
    - "^prod-metrics\\..*"
tags:
  - "bigtable"
  - "nosql"

```

## Configuration
The following configuration options are available:

| Property | Type | Required | Description |
|----------|------|----------|-------------|
| credentials | GCPCredentials | false | GCP credentials configuration |
| emulator_host | string | false | host:port of a Bigtable emulator. Connects without credentials and requires instances to be listed |
| external_links | []ExternalLink | false | External links to show on all assets |
| filter | Filter | false | Filter discovered assets by name (regex) |
| include_backups | bool | false | Count the backups held by each instance |
| include_columns | bool | false | Read a sample of rows to find the columns each table holds |
| include_statistics | bool | false | Count the rows in each table. This reads the whole table |
| instances | []string | false | Instance IDs to discover. Leave empty to discover every instance in the project |
| max_count_rows | int | false | Give up counting a table after this many rows |
| project_id | string | true | Google Cloud project ID |
| sample_rows | int | false | Rows to read per table when finding columns |
| tags | TagsConfig | false | Tags to apply to discovered assets |

## Available Metadata

The following metadata fields are available:

| Field | Type | Description |
|-------|------|-------------|
| backup_count | int | Number of backups held by the instance's clusters |
| change_stream_retention | string | How long change data is retained |
| clusters | []BigtableCluster | Clusters serving the instance |
| column_families | []string | Column families defined on the table |
| column_family | string | Column family the column belongs to |
| column_name | string | Column name in Bigtable's family:qualifier notation |
| data_type | string | Storage type, always bytes |
| deletion_protection | bool | Whether the table is protected against deletion |
| display_name | string | Instance display name |
| emulator | bool | Whether the instance was read from an emulator |
| gc_policies | map[string]string | Garbage collection rule per column family |
| inferred_type | string | What the sampled values looked like (text, int64, binary) |
| instance_id | string | Instance the table belongs to |
| instance_type | string | Instance type (PRODUCTION, DEVELOPMENT) |
| is_nullable | bool | Whether the column may be missing from a row |
| is_primary_key | bool | Whether the column is the row key |
| labels | map[string]string | Labels set on the instance |
| occurrence | int | Number of sampled rows that held the column |
| project_id | string | Google Cloud project holding the table |
| qualifier | string | Qualifier within the column family |
| sampled_rows | int | Number of rows read to find the table's columns |
| state | string | Instance state (READY, CREATING) |
| table_count | int | Number of tables in the instance |
| table_name | string | Table name within the instance |
| url | string | Google Cloud Console link to the table |
