---
title: Kafka
description: Discover Kafka topics from Kafka clusters
status: experimental
---

# Kafka

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


Marmot plugin for Apache Kafka. Discovers topics from Kafka clusters, captures topic configurations and partition details and optionally enriches assets with schemas from a Confluent Schema Registry.

Marmot plugins are standalone binaries that the Marmot host launches on demand via [go-plugin](https://github.com/hashicorp/go-plugin) and talks to over gRPC. It is built on the [Marmot plugin SDK](https://github.com/marmotdata/plugin-sdk).

> Looking for a managed service? Marmot has dedicated plugins for [Confluent Cloud](../confluent) and [Redpanda](../redpanda) with pre-configured defaults.

## Connection Examples

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

### Self-Hosted with mTLS

```yaml
bootstrap_servers: "kafka-1.internal:9093"
client_id: "marmot-discovery"
tls:
  enabled: true
  ca_cert_path: "/etc/kafka/ca.pem"
  cert_path: "/etc/kafka/client.pem"
  key_path: "/etc/kafka/client-key.pem"
```

### Local development (no auth)

```yaml
bootstrap_servers: "localhost:9092"
client_id: "marmot-discovery"
tls:
  enabled: false
```

## Schema Registry

Enable Schema Registry to enrich discovered topics with their value and key schemas:

```yaml
schema_registry:
  enabled: true
  url: "https://schema-registry.prod.com"
  config:
    basic.auth.user.info: "sr-key:sr-secret"
```

Schemas for subjects matching `{topic}-value`, `{topic}-key` or other `{topic}-*` patterns are pulled from the registry and attached to the topic asset.

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

## Development

Build and test:

```sh
make build
make test
```

To run a local build inside Marmot:

```sh
make install
```

This copies the binary to `~/.marmot/plugins/`, the directory Marmot scans for local plugins. A local plugin shadows the released core plugin with the same name: Marmot skips downloading it and loads your build instead. Delete the binary from `~/.marmot/plugins/` to fall back to the released version.

If your Marmot runs with a custom plugins directory (`MARMOT_PLUGINS_DIR`), set the same value for `make install` so both point at the same place.



## Configuration
The following configuration options are available:

| Property | Type | Required | Description |
|----------|------|----------|-------------|
| tags | multiselect | false | Tags to apply to discovered assets |
| external_links | []object | false | External links to show on all assets |
| external_links.name | string | true | Display name for the link |
| external_links.icon | string | false | Icon identifier for the link |
| external_links.url | string | true | URL to the external resource |
| filter | object | false | Filter discovered assets by name (regex) |
| filter.include | multiselect | false | Include patterns for resource names (regex) |
| filter.exclude | multiselect | false | Exclude patterns for resource names (regex) |
| bootstrap_servers | string | true | Comma-separated list of bootstrap servers |
| client_id | string | false | Client ID for the consumer |
| authentication | object | false | Authentication configuration |
| authentication.type | select | false | Authentication type: none, sasl_plaintext, sasl_ssl, ssl |
| authentication.username | string | false | SASL username |
| authentication.password | password | false | SASL password |
| authentication.mechanism | select | false | SASL mechanism: PLAIN, SCRAM-SHA-256, SCRAM-SHA-512 |
| consumer_config | string | false | Additional consumer configuration |
| client_timeout_seconds | int | false | Request timeout in seconds |
| tls | object | false | TLS configuration |
| tls.enabled | bool | false | Whether to enable TLS |
| tls.cert_path | string | false | Path to TLS certificate file |
| tls.key_path | string | false | Path to TLS key file |
| tls.ca_cert_path | string | false | Path to TLS CA certificate file |
| tls.skip_verify | bool | false | Skip TLS verification |
| schema_registry | object | false | Schema Registry configuration |
| schema_registry.url | string | false | Schema Registry URL |
| schema_registry.config | string | false | Additional Schema Registry configuration |
| schema_registry.enabled | bool | false | Whether to use Schema Registry |
| schema_registry.skip_verify | bool | false | Skip TLS certificate verification |
| include_partition_info | bool | false | Whether to include partition information in metadata |
| include_topic_config | bool | false | Whether to include topic configuration in metadata |

## Available Metadata

### Topic

| Field | Type | Description |
|-------|------|-------------|
| topic_name | string | Name of the Kafka topic |
| partition_count | int32 | Number of partitions |
| replication_factor | int16 | Replication factor |
| retention_ms | string | Message retention period in milliseconds |
| retention_bytes | string | Maximum size of the topic in bytes |
| cleanup_policy | string | Topic cleanup policy |
| min_insync.replicas | string | Minimum number of in-sync replicas |
| max_message.bytes | string | Maximum message size in bytes |
| segment_bytes | string | Segment file size in bytes |
| segment_ms | string | Segment file roll time in milliseconds |
| delete_retention_ms | string | Time to retain deleted segments in milliseconds |
| value_schema_id | int | ID of the value schema in Schema Registry |
| value_schema_version | int | Version of the value schema |
| value_schema_type | string | Type of the value schema (AVRO, JSON, etc.) |
| value_schema | string | Value schema definition |
| key_schema_id | int | ID of the key schema in Schema Registry |
| key_schema_version | int | Version of the key schema |
| key_schema_type | string | Type of the key schema (AVRO, JSON, etc.) |
| key_schema | string | Key schema definition |

### Consumer Group

| Field | Type | Description |
|-------|------|-------------|
| group_id | string | Consumer group ID |
| state | string | Current state of the consumer group |
| protocol | string | Rebalance protocol |
| protocol_type | string | Protocol type |
| subscribed_topics | []string | Topics the group is subscribed to |
| members | []string | Members of the consumer group |