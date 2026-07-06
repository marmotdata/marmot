# marmot-plugin-kafka

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

## Configuration

The following configuration options are available:

| Property | Type | Required | Description |
|----------|------|----------|-------------|
| `authentication` | AuthConfig | false | Authentication configuration |
| `bootstrap_servers` | string | true | Comma-separated list of bootstrap servers |
| `client_id` | string | false | Client ID for the consumer |
| `client_timeout_seconds` | int | false | Request timeout in seconds |
| `consumer_config` | map[string]string | false | Additional consumer configuration |
| `external_links` | []ExternalLink | false | External links to show on all assets |
| `filter` | Filter | false | Filter discovered assets by name (regex) |
| `include_partition_info` | bool | false | Whether to include partition information in metadata (default `true`) |
| `include_topic_config` | bool | false | Whether to include topic configuration in metadata (default `true`) |
| `schema_registry` | SchemaRegistryConfig | false | Schema Registry configuration |
| `tags` | TagsConfig | false | Tags to apply to discovered assets |
| `tls` | TLSConfig | false | TLS configuration |

### AuthConfig

| Property | Type | Description |
|----------|------|-------------|
| `type` | string | Authentication type: `none`, `sasl_plaintext`, `sasl_ssl` or `ssl` |
| `username` | string | SASL username |
| `password` | string | SASL password (sensitive) |
| `mechanism` | string | SASL mechanism: `PLAIN`, `SCRAM-SHA-256` or `SCRAM-SHA-512` |

### TLSConfig

| Property | Type | Description |
|----------|------|-------------|
| `enabled` | bool | Whether to enable TLS |
| `cert_path` | string | Path to TLS certificate file |
| `key_path` | string | Path to TLS key file |
| `ca_cert_path` | string | Path to TLS CA certificate file |
| `skip_verify` | bool | Skip TLS verification |

### SchemaRegistryConfig

| Property | Type | Description |
|----------|------|-------------|
| `url` | string | Schema Registry URL |
| `enabled` | bool | Whether to use Schema Registry |
| `config` | map[string]string | Additional Schema Registry configuration |
| `skip_verify` | bool | Skip TLS certificate verification |

## Available Metadata

The following metadata fields are available on discovered topic assets:

| Field | Type | Description |
|-------|------|-------------|
| `topic_name` | string | Name of the Kafka topic |
| `partition_count` | int32 | Number of partitions |
| `replication_factor` | int16 | Replication factor |
| `retention_ms` | string | Message retention period in milliseconds |
| `retention_bytes` | string | Maximum size of the topic in bytes |
| `cleanup_policy` | string | Topic cleanup policy |
| `min_insync_replicas` | string | Minimum number of in-sync replicas |
| `max_message_bytes` | string | Maximum message size in bytes |
| `segment_bytes` | string | Segment file size in bytes |
| `segment_ms` | string | Segment file roll time in milliseconds |
| `delete_retention_ms` | string | Time to retain deleted segments in milliseconds |
| `value_schema` | string | Value schema definition |
| `value_schema_id` | int | ID of the value schema in Schema Registry |
| `value_schema_type` | string | Type of the value schema (AVRO, JSON, etc.) |
| `value_schema_version` | int | Version of the value schema |
| `key_schema` | string | Key schema definition |
| `key_schema_id` | int | ID of the key schema in Schema Registry |
| `key_schema_type` | string | Type of the key schema (AVRO, JSON, etc.) |
| `key_schema_version` | int | Version of the key schema |

Consumer group assets carry the following fields:

| Field | Type | Description |
|-------|------|-------------|
| `group_id` | string | Consumer group ID |
| `state` | string | Current state of the consumer group |
| `protocol` | string | Rebalance protocol |
| `protocol_type` | string | Protocol type |
| `subscribed_topics` | []string | Topics the group is subscribed to |
| `members` | []string | Members of the consumer group |

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
