---
title: Kafka
description: This plugin discovers Kafka topics from Kafka clusters.
status: experimental
---

# Kafka

**Status:** experimental

The Kafka plugin discovers and catalogs Kafka topics from Kafka clusters. It captures topic configurations, partition details, schema information from Schema Registry, and supports various authentication methods including SASL and TLS.

## Connection Examples

### Confluent Cloud

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
schema_registry:
  url: "https://psrc-xxxxx.us-west-2.aws.confluent.cloud"
  enabled: true
  config:
    basic.auth.user.info: "sr-key:sr-secret"
tags:
  - "confluent"
```

### Redpanda Cloud

```yaml
bootstrap_servers: "seed-xxxxx.cloud.redpanda.com:9092"
client_id: "marmot-discovery"
authentication:
  type: "sasl_ssl"
  username: "your-username"
  password: "your-password"
  mechanism: "SCRAM-SHA-256"
tls:
  enabled: true
tags:
  - "redpanda"
```

### Self-Hosted with SASL

```yaml
bootstrap_servers: "kafka-1.prod.com:9092,kafka-2.prod.com:9092"
client_id: "marmot-discovery"
authentication:
  type: "sasl_ssl"
  username: "your-username"
  password: "your-password"
  mechanism: "SCRAM-SHA-512"
tls:
  enabled: true
  ca_cert_path: "/path/to/ca.pem"
  cert_path: "/path/to/client.pem"
  key_path: "/path/to/client-key.pem"
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
| include_partition_info | bool | false | Whether to include partition information in metadata |
| include_topic_config | bool | false | Whether to include topic configuration in metadata |
| schema_registry | SchemaRegistryConfig | false | Schema Registry configuration |
| tags | TagsConfig | false | Tags to apply to discovered assets |
| tls | TLSConfig | false | TLS configuration |
| topic_filter | plugin.Filter | false | Filter configuration for topics |

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