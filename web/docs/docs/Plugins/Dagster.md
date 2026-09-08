---
title: Dagster
description: Discovers jobs, ops and software-defined assets from Dagster.
status: experimental
---

# Dagster

<div class="flex flex-col gap-3 mb-6 pb-6 border-b border-gray-200">
<div class="flex items-center gap-3">
<span class="inline-flex items-center rounded-full px-4 py-2 text-sm font-medium bg-earthy-yellow-300 text-earthy-yellow-900">Experimental</span>
</div>
<div class="flex items-center gap-2">
<span class="text-sm text-gray-500">Creates:</span>
<div class="flex flex-wrap gap-2"><span class="inline-flex items-center rounded-lg px-4 py-2 text-sm font-medium bg-earthy-green-100 text-earthy-green-800 border border-earthy-green-300">Assets</span><span class="inline-flex items-center rounded-lg px-4 py-2 text-sm font-medium bg-earthy-green-100 text-earthy-green-800 border border-earthy-green-300">Lineage</span><span class="inline-flex items-center rounded-lg px-4 py-2 text-sm font-medium bg-earthy-green-100 text-earthy-green-800 border border-earthy-green-300">Run History</span></div>
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


The Dagster plugin discovers jobs, the ops inside them, and software-defined assets from a Dagster webserver. Everything is read through the GraphQL endpoint at `{host}/graphql`, so the same configuration works against open source Dagster and Dagster+.

## Prerequisites

- A reachable Dagster webserver
- For Dagster+, an API token with read access. The plugin sends it as the `Dagster-Cloud-Api-Token` header.

## What it creates

- A `Pipeline` per job, named after the job. Schedules, sensors and a summary of the recent runs are attached as metadata. Dagster's implicit asset jobs, the ones named `__ASSET_JOB`, are skipped.
- A `Task` per op, named `<job>/<op handle>`.
- A `Dataset` per software-defined asset, named after its asset key joined with `/`. A `TableSchema` metadata entry becomes the asset's columns, and any other metadata a user attached is flattened alongside the fields below.

Pipelines contain their Tasks, Tasks depend on the Tasks feeding their inputs, Datasets feed the Datasets built from them, and a Pipeline produces every Dataset it materialises.

:::tip[Warehouse lineage]
When an asset's compute kind is Snowflake, BigQuery, Postgres or DuckDB and it names a full database, schema and table, the Dataset also produces that native table. The edge lands only if the matching plugin has already discovered the table.
:::

## Example Configuration

```yaml

host: "http://localhost:3000"
token: "${DAGSTER_CLOUD_API_TOKEN}"
verify_ssl: true
include_ops: true
include_assets: true
include_run_history: true
run_history_limit: 10
code_locations:
  - "analytics"
filter:
  include:
    - "^orders_.*"
  exclude:
    - ".*_test$"
tags:
  - "dagster"
  - "orchestration"

```

## Configuration
The following configuration options are available:

| Property | Type | Required | Description |
|----------|------|----------|-------------|
| code_locations | []string | false | Only discover these code locations. Empty means all of them |
| external_links | []ExternalLink | false | External links to show on all assets |
| filter | Filter | false | Filter discovered assets by name (regex) |
| host | string | true | Dagster webserver URL, for example http://localhost:3000 |
| include_assets | bool | false | Discover software-defined assets as Dataset assets |
| include_ops | bool | false | Discover the ops inside each job as Task assets |
| include_run_history | bool | false | Collect recent runs of each job |
| run_history_limit | int | false | How many recent runs to read per job |
| tags | TagsConfig | false | Tags to apply to discovered assets |
| token | string | false | Dagster+ API token |
| verify_ssl | bool | false | Check the server's TLS certificate |

## Available Metadata

The following metadata fields are available:

| Field | Type | Description |
|-------|------|-------------|
| asset_key | []string | Asset key path as declared in Dagster |
| code_location | string | Code location serving the job or asset |
| column_name | string | Column name |
| compute_kind | string | Technology that computes the asset, for example duckdb |
| data_type | string | Column data type as declared in the table schema |
| description | string | Job, op or asset description |
| group | string | Asset group the asset belongs to |
| handle_id | string | Path to the op within the job graph |
| input_count | int | Number of inputs the op declares |
| is_asset_job | bool | Whether the job exists to materialise assets |
| is_nullable | bool | Whether null values are allowed |
| is_partitioned | bool | Whether the asset is partitioned |
| job | string | Job the op belongs to |
| job_id | string | Dagster's internal identifier for the job |
| job_names | []string | Jobs that can materialise the asset |
| last_materialization_run | string | Run that produced the most recent materialization |
| last_materialized_at | string | Time of the most recent materialization |
| last_run_at | string | Start time of the most recent run |
| last_run_status | string | Status of the most recent run |
| op | string | Name of the op definition |
| op_count | int | Number of ops in the job |
| op_names | []string | Ops that compute the asset |
| output_count | int | Number of outputs the op declares |
| repository | string | Repository the job or asset is defined in |
| run_count | int | Number of runs read for this job |
| schedules | []object | Schedules that launch the job, with cron expression and status |
| sensors | []object | Sensors that launch the job, with status |
| success_rate | float64 | Percentage of finished runs that succeeded |
| tags | object | Job tags, keyed by tag name |
| url | string | Link to the job, op or asset in the Dagster UI |
