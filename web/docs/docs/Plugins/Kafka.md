---
title: Kafka
description: This plugin discovers Kafka topics from Kafka clusters.
status: experimental
---

# Kafka

This plugin discovers Kafka topics from Kafka clusters.

**Status:** experimental

## Example Configuration

```yaml

bootstrap_servers: "localhost:9092"
client_id: "marmot-kafka-plugin"
client_timeout_seconds: 60
authentication:
  type: "sasl_plaintext"
  username: "username"
  password: "password"
  mechanism: "PLAIN"
schema_registry:
  url: "http://localhost:8081"
  enabled: true
  config:
    basic.auth.user.info: "username:password"
topic_filter:
  include:
    - "^prod-.*"
    - "^staging-.*"
  exclude:
    - ".*-test$"
    - ".*-dev$"
include_partition_info: true
include_topic_config: true
tags:
  - "kafka"
  - "messaging"

```

## Configuration
The following configuration options are available:

| Property | Type | Required | Description |
|----------|------|----------|-------------|
| bootstrap_servers | string | false | Comma-separated list of bootstrap servers |
| client_id | string | false | Client ID for the consumer |
| authentication | AuthConfig | false | Authentication configuration |
| consumer_config | map[string]string | false | Additional consumer configuration |
| client_timeout_seconds | int | false | Request timeout in seconds |
| schema_registry | SchemaRegistryConfig | false | Schema Registry configuration |
| topic_filter | plugin.Filter | false | Filter configuration for topics |
| include_partition_info | bool | false | Whether to include partition information in metadata |
| include_topic_config | bool | false | Whether to include topic configuration in metadata |

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