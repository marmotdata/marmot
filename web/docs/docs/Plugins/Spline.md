---
title: Spline
description: Discovers Spark applications and their lineage from a Spline server.
status: experimental
---

# Spline

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


The Spline plugin discovers Spark applications and their lineage from a Spline server. It reads the Spline consumer API, so it needs no access to the Spark cluster itself.

## One Pipeline Per Application

A Pipeline asset is created per Spark **application name**, not per execution event. A job that runs nightly stays a single asset whose run history grows, instead of producing a new asset on every Spark run. Every run in the window becomes a run history event, with a failed run recorded as `FAIL`.

## Lineage

Spline records the URI Spark read from or wrote to. The plugin resolves those URIs to the assets other Marmot plugins publish, so a Spark job links to the real table or bucket rather than to a copy of it. Each input becomes a `Table FEEDS Pipeline` edge and the output a `Pipeline PRODUCES Table` edge.

| URI | Resolves to |
|-----|-------------|
| `jdbc:postgresql://host:port/db:schema.table` | PostgreSQL table |
| `jdbc:mysql://host/db:table`, `jdbc:mariadb://...` | MySQL or MariaDB table |
| `jdbc:sqlserver://host;databaseName=db:schema.table` | SQL Server `db.schema.table` |
| `jdbc:oracle:thin:@//host:port/service:schema.table` | Oracle `schema.table` |
| `jdbc:redshift://...`, `jdbc:snowflake://...` | Redshift or Snowflake `db.schema.table` |
| `s3://bucket/key`, `s3a://`, `s3n://` | S3 bucket |
| `gs://bucket/key` | Google Cloud Storage bucket |
| `abfss://container@account/...` | Azure Blob container |
| `hive://db/table` or a bare `db.table` | Hive table |
| `delta://path/to/table` | Delta Lake table |

The table can be appended to a JDBC URI either after a colon or as a `table` query parameter; both are read. Anything else, including `hdfs://` and local file paths, produces no edge. The raw URIs are always kept in the `inputs` and `outputs` metadata.

The plugin creates no tables or buckets of its own, so an edge only shows up in Marmot once the plugin that owns the other end has catalogued the asset.

## Column Lineage

With `include_column_lineage` the plugin reads Spline's attribute lineage for every column a run produced and stores it on the pipeline under the `column_lineage` metadata key as JSON, keyed by the MRN of the table produced. It costs one request per column, capped at 200 columns per execution plan.

## Example Configuration

```yaml

host: "http://spline.internal:8080"
ui_host: "http://spline.internal:9090"
days: 7
max_events: 1000
include_column_lineage: true
tags:
  - "spline"
  - "spark"

```

`host` is the Spline REST gateway root. A `/consumer` or `/producer` suffix is stripped, so pasting either API's URL works.

## Configuration
The following configuration options are available:

| Property | Type | Required | Description |
|----------|------|----------|-------------|
| days | int | false | Only ingest Spark runs from the last N days |
| external_links | []ExternalLink | false | External links to show on all assets |
| filter | Filter | false | Filter discovered assets by name (regex) |
| host | string | true | Spline REST gateway URL, for example http://spline:8080 |
| include_column_lineage | bool | false | Read column level lineage for each run |
| max_events | int | false | Maximum number of Spark runs to read |
| page_size | int | false | Number of runs to read per request |
| password | string | false | Password for basic authentication |
| tags | TagsConfig | false | Tags to apply to discovered assets |
| token | string | false | Bearer token, as an alternative to basic authentication |
| ui_host | string | false | Spline UI URL, used for links back to a run |
| username | string | false | Username for basic authentication |
| verify_ssl | bool | false | Verify the server's TLS certificate |

## Available Metadata

The following metadata fields are available:

| Field | Type | Description |
|-------|------|-------------|
| agent_name | string | Spline agent that captured the run |
| agent_version | string | Version of the Spline agent |
| application_ids | []string | Recent Spark application ids, newest first |
| column_lineage | string | Column level lineage as JSON, keyed by the MRN of the table produced |
| execution_count | int | Number of runs seen in the ingest window |
| execution_plan_ids | []string | Recent Spline execution plan ids, newest first |
| framework | string | Framework that ran the application, for example spark 3.5.0 |
| inputs | []string | Raw data source URIs the application read |
| last_duration_ms | int64 | Duration of the most recent run in milliseconds |
| last_error | string | Error message of the most recent run, when it failed |
| last_execution_at | string | When the most recent run finished |
| last_execution_id | string | Spline execution event id of the most recent run |
| outputs | []string | Raw data source URIs the application wrote |
| system_name | string | System that produced the execution plan, for example spark |
| system_version | string | Version of that system |
| url | string | Link to the most recent run in the Spline UI |

Each run history event carries these facets:

| Field | Type | Description |
|-------|------|-------------|
| append | bool | Whether the run appended to the output instead of overwriting it |
| application_id | string | Spark application id of the run |
| duration_ms | int64 | Duration of the run in milliseconds |
| execution_plan_id | string | Spline execution plan the run used |
| output | string | Data source URI the run wrote to |
