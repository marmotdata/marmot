---
title: Confluent Cloud
description: Discover Kafka topics from Confluent Cloud clusters.
status: experimental
---

# Confluent Cloud

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


The Confluent Cloud plugin discovers Kafka topics from Confluent Cloud clusters. It uses the same discovery engine as the Kafka plugin with defaults tuned for Confluent Cloud.

## Connection

Confluent Cloud requires SASL/SSL authentication with an API key pair. You can create API keys in the Confluent Cloud Console.

```yaml
bootstrap_servers: "pkc-xxxxx.us-west-2.aws.confluent.cloud:9092"
client_id: "marmot-discovery"
authentication:
  type: "sasl_ssl"
  username: "your-api-key"
  password: "your-api-secret"
  mechanism: "PLAIN"
tls:
  enabled: true
```

## Schema Registry

If your Confluent Cloud environment has Schema Registry enabled, add the following to pull schema metadata:

```yaml
schema_registry:
  url: "https://psrc-xxxxx.us-west-2.aws.confluent.cloud"
  enabled: true
  config:
    basic.auth.user.info: "sr-key:sr-secret"
```



## Example Configuration

```yaml

bootstrap_servers: "kafka-1.prod.com:9092,kafka-2.prod.com:9092"
client_id: "marmot-discovery"
authentication:
  type: "sasl_ssl"
  username: "your-api-key"
  password: "your-api-secret"
  mechanism: "PLAIN"
tls:
  enabled: true
tags:
  - "kafka"
  - "streaming"

```

## Configuration
The following configuration options are available:

| Property | Type | Required | Description |
|----------|------|----------|-------------|
| authentication | AuthConfig | false | Authentication configuration |
| bootstrap_servers | string | false | Comma-separated list of bootstrap servers |
| client_id | string | false | Client ID for the consumer |
| client_timeout_seconds | int | false | Request timeout in seconds |
| consumer_config | map[string]string | false | Additional consumer configuration |
| external_links | []ExternalLink | false | External links to show on all assets |
| filter | Filter | false | Filter discovered assets by name (regex) |
| include_partition_info | bool | false | Whether to include partition information in metadata |
| include_topic_config | bool | false | Whether to include topic configuration in metadata |
| schema_registry | SchemaRegistryConfig | false | Schema Registry configuration |
| tags | TagsConfig | false | Tags to apply to discovered assets |
| tls | TLSConfig | false | TLS configuration |

## Available Metadata

The following metadata fields are available:

| Field | Type | Description |
|-------|------|-------------|
| cleanup_policy | string | Topic cleanup policy |
| delete_retention_ms | string | Time to retain deleted segments in milliseconds |
| group_id | string | Consumer group ID |
| key_schema | string | Key schema definition |
| key_schema_id | int | ID of the key schema in Schema Registry |
| key_schema_type | string | Type of the key schema (AVRO, JSON, etc.) |
| key_schema_version | int | Version of the key schema |
| max_message_bytes | string | Maximum message size in bytes |
| members | []string | Members of the consumer group |
| min_insync_replicas | string | Minimum number of in-sync replicas |
| partition_count | int32 | Number of partitions |
| protocol | string | Rebalance protocol |
| protocol_type | string | Protocol type |
| replication_factor | int16 | Replication factor |
| retention_bytes | string | Maximum size of the topic in bytes |
| retention_ms | string | Message retention period in milliseconds |
| segment_bytes | string | Segment file size in bytes |
| segment_ms | string | Segment file roll time in milliseconds |
| state | string | Current state of the consumer group |
| subscribed_topics | []string | Topics the group is subscribed to |
| topic_name | string | Name of the Kafka topic |
| value_schema | string | Value schema definition |
| value_schema_id | int | ID of the value schema in Schema Registry |
| value_schema_type | string | Type of the value schema (AVRO, JSON, etc.) |
| value_schema_version | int | Version of the value schema |