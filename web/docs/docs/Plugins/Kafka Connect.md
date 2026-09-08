---
title: Kafka Connect
description: Discovers connectors, tasks and topics from Kafka Connect clusters with lineage to the systems they move data between.
status: experimental
---

# Kafka Connect

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


The Kafka Connect plugin discovers connectors, their tasks and the topics they move data through from a Kafka Connect cluster. It talks to the Connect REST API, so it works with self-hosted workers and Confluent Cloud alike.

Every connector becomes a Pipeline and every task a Task under it. The topics a connector reads or writes become Topic assets with the same identity the Kafka plugin uses, so a topic both plugins see is one asset with two sources.

## Lineage

Source connectors are linked from the tables or collections they capture and to the topics they write. Sink connectors are linked from the topics they read and to the tables, collections or buckets they write. Those dataset edges point at the asset the owning Marmot plugin creates (PostgreSQL, MySQL, S3 and so on); this plugin never creates tables or buckets itself, only topics.

Dataset edges are derived from the connector config for Debezium (PostgreSQL, MySQL, SQL Server, MongoDB, Oracle) and the matching Confluent Cloud CDC connectors, the Confluent JDBC source and sink, the S3, GCS and Azure Blob sinks, the Snowflake, BigQuery and Elasticsearch sinks, and the MongoDB source and sink. Other connector classes get topic edges only.

Topics come from the worker's active topics endpoint (KIP-558, Kafka 2.5 and later) when it has seen records for the connector. Otherwise they are derived from the config, with RegexRouter transforms applied.

With `include_topics: false` the topic edges are still emitted; they only land on topics the Kafka plugin has catalogued.

:::tip[Credentials]
The connector config is stored in the Pipeline metadata with every credential value masked (any key containing `password`, `secret`, `token`, `credential`, `sasl.jaas`, `key.id`, `access.key` or `private`). Set `include_config: false` to leave the config out entirely.
:::



## Example Configuration

```yaml

host: "http://connect.internal:8083"
username: "marmot"
password: "connect_secure_pass"
verify_ssl: true
include_tasks: true
include_topics: true
include_config: true
discover_lineage: true
tags:
  - "kafka-connect"

```

## Configuration
The following configuration options are available:

| Property | Type | Required | Description |
|----------|------|----------|-------------|
| discover_lineage | bool | false | Whether to link connectors to the topics and datasets they move data between |
| external_links | []ExternalLink | false | External links to show on all assets |
| filter | Filter | false | Filter discovered assets by name (regex) |
| host | string | true | Kafka Connect REST URL (e.g. http://connect:8083) |
| include_config | bool | false | Whether to store each connector's config in its metadata, with credentials masked |
| include_tasks | bool | false | Whether to discover connector tasks as Task assets |
| include_topics | bool | false | Whether to discover the topics connectors read and write as Topic assets |
| password | string | false | Password for basic authentication |
| tags | TagsConfig | false | Tags to apply to discovered assets |
| username | string | false | Username for basic authentication |
| verify_ssl | bool | false | Whether to verify the TLS certificate of the Connect REST endpoint |

## Available Metadata

The following metadata fields are available:

| Field | Type | Description |
|-------|------|-------------|
| config | map[string]string | Connector config with credential values masked |
| connect_version | string | Version of the Connect worker |
| connector | string | Name of the connector the task belongs to |
| connector_class | string | Java class of the connector |
| connector_type | string | Connector direction (source, sink) |
| consumers | []string | Sink connectors reading from the topic |
| description | string | Description from the connector config, when set |
| error | string | First 500 characters of the failure trace, when the connector or a task has failed |
| kafka_cluster_id | string | Id of the Kafka cluster the worker is attached to |
| plugin_version | string | Version of the installed connector plugin |
| producers | []string | Source connectors writing to the topic |
| state | string | Connector or task state (RUNNING, PAUSED, FAILED, UNASSIGNED) |
| task_count | int | Number of tasks the connector is split into |
| task_id | int | Task id within the connector |
| task_states | map[string]string | State of each task, keyed by task id |
| topic_name | string | Topic name |
| topics | []string | Topics the connector reads or writes |
| url | string | REST URL of the connector |
| worker_id | string | Worker the connector or task runs on |
