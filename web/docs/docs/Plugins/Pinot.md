---
title: Pinot
description: Discovers tables, schemas and stream lineage from Apache Pinot clusters.
status: experimental
---

# Pinot

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


The Pinot plugin discovers tables from Apache Pinot clusters through the controller REST API. Each logical table becomes one Table asset, whether it is offline, realtime or hybrid. The schema is flattened into columns that keep their Pinot role (dimension, metric or datetime), and the table's time column, retention, tenants, indexes and segment counts are recorded as metadata.

Row counts come from a `COUNT(*)` query and sizes from the controller's size report. Realtime tables that consume from Kafka or Kinesis are linked to the topic or stream with a FEEDS edge, using the same identity the Kafka and Kinesis plugins produce.

## Connection

`controller_url` is required. Queries go through the controller's `/sql` proxy unless `broker_url` is set. Basic auth and bearer tokens are supported; set `database` to discover one logical database on Pinot 1.1 or later.



## Example Configuration

```yaml

controller_url: "http://pinot-controller.internal:9000"
broker_url: "http://pinot-broker.internal:8099"
username: "marmot"
password: "${PINOT_PASSWORD}"
include_columns: true
include_row_counts: true
include_sizes: true
discover_lineage: true
tags:
  - "pinot"
  - "analytics"

```

## Configuration
The following configuration options are available:

| Property | Type | Required | Description |
|----------|------|----------|-------------|
| broker_url | string | false | Pinot broker URL for queries. When empty, queries go through the controller |
| controller_url | string | true | Pinot controller URL, for example http://localhost:9000 |
| database | string | false | Logical database to discover (Pinot 1.1 and later). Empty means the default database |
| discover_lineage | bool | false | Whether to link realtime tables to the Kafka topic or Kinesis stream that feeds them |
| exclude_system_tables | bool | false | Whether to skip tables whose name starts with an underscore |
| external_links | []ExternalLink | false | External links to show on all assets |
| filter | Filter | false | Filter discovered assets by name (regex) |
| include_columns | bool | false | Whether to include column information in table metadata |
| include_row_counts | bool | false | Whether to count rows with a COUNT(*) query per table |
| include_sizes | bool | false | Whether to include the segment size reported by the controller |
| password | string | false | Password for basic authentication |
| tags | TagsConfig | false | Tags to apply to discovered assets |
| token | string | false | Bearer token, used instead of a username and password |
| username | string | false | Username for basic authentication |
| verify_ssl | bool | false | Whether to verify the TLS certificate of the controller and broker |

## Available Metadata

The following metadata fields are available:

| Field | Type | Description |
|-------|------|-------------|
| broker_tenant | string | Broker tenant serving the table |
| column_name | string | Column name |
| data_type | string | Pinot data type; multi-value columns carry a [] suffix |
| default_null_value | any | Value stored in place of null |
| field_type | string | Role of the field in the schema: dimension, metric or datetime |
| format | string | Date-time format, for example 1:DAYS:EPOCH or TIMESTAMP |
| granularity | string | Date-time granularity, for example 1:SECONDS |
| ingestion_type | string | How data arrives: batch, stream or hybrid |
| inverted_index_columns | []string | Columns with an inverted index |
| is_dim_table | bool | Whether the table is a dimension table replicated to every server |
| is_nullable | bool | Whether null values are allowed (true unless the schema marks the field notNull) |
| is_primary_key | bool | Whether the column is part of the primary key |
| load_mode | string | How segments are loaded on servers (MMAP or HEAP) |
| offline_segment_count | int | Number of offline segments |
| pinot_version | string | Pinot release reported by the controller |
| primary_key_columns | []string | Primary key columns declared in the schema |
| realtime_segment_count | int | Number of realtime segments, including the consuming one |
| replication | string | Number of replicas per segment |
| retention | string | How long segments are kept, as value and unit (for example 30 DAYS) |
| schema_name | string | Name of the Pinot schema the table config points at, when it differs from the table name |
| segment_count | int | Total number of segments across table types |
| server_tenant | string | Server tenant hosting the table |
| single_value | bool | Whether each row holds one value rather than an array |
| sorted_column | string | Column segments are sorted by |
| stream_brokers | string | Broker list a realtime table consumes from |
| stream_topic | string | Topic or stream name a realtime table consumes from |
| stream_type | string | Stream type a realtime table consumes from, for example kafka or kinesis |
| table_name | string | Logical table name, without the _OFFLINE or _REALTIME suffix |
| table_types | []string | Table types present: OFFLINE, REALTIME or both for a hybrid table |
| time_column | string | Column segments are partitioned and retained by |
| time_type | string | Unit of the time column, for example DAYS or MILLISECONDS |
| url | string | Link to the table in the controller UI |
