# marmot-plugin-confluent

Marmot plugin for Confluent Cloud. Discovers Kafka topics from Confluent Cloud clusters using the same discovery engine as the [Kafka plugin](../kafka), with defaults tuned for Confluent Cloud.

Marmot plugins are standalone binaries that the Marmot host launches on demand via [go-plugin](https://github.com/hashicorp/go-plugin) and talks to over gRPC. It is built on the [Marmot plugin SDK](https://github.com/marmotdata/plugin-sdk).

## Connection

Confluent Cloud requires SASL/SSL authentication with an API key pair. You can create API keys in the Confluent Cloud Console.

```yaml
bootstrap_servers: "pkc-xxxxx.us-west-2.aws.confluent.cloud:9092"
client_id: "marmot-discovery"
authentication:
  username: "your-api-key"
  password: "your-api-secret"
```

TLS is enabled by default and SASL type and mechanism are locked to Confluent Cloud's requirements. Any config field the Confluent Cloud plugin does not expose is inherited from the Kafka plugin's shared config surface.

## Schema Registry

If your Confluent Cloud environment has Schema Registry enabled, add the following to pull schema metadata:

```yaml
schema_registry:
  enabled: true
  url: "https://psrc-xxxxx.us-west-2.aws.confluent.cloud"
  config:
    basic.auth.user.info: "sr-key:sr-secret"
```

## Example Configuration

```yaml
bootstrap_servers: "pkc-xxxxx.us-west-2.aws.confluent.cloud:9092"
client_id: "marmot-discovery"
authentication:
  username: "your-api-key"
  password: "your-api-secret"
schema_registry:
  enabled: true
  url: "https://psrc-xxxxx.us-west-2.aws.confluent.cloud"
  config:
    basic.auth.user.info: "sr-key:sr-secret"
tags:
  - "confluent"
  - "streaming"
```

## Configuration

The following configuration options are available:

| Property | Type | Required | Description |
|----------|------|----------|-------------|
| `authentication` | AuthConfig | false | Authentication credentials |
| `bootstrap_servers` | string | true | Confluent Cloud bootstrap servers |
| `client_id` | string | false | Client ID for the consumer |
| `client_timeout_seconds` | int | false | Request timeout in seconds |
| `external_links` | []ExternalLink | false | External links to show on all assets |
| `filter` | Filter | false | Filter discovered assets by name (regex) |
| `include_partition_info` | bool | false | Whether to include partition information in metadata (default `true`) |
| `include_topic_config` | bool | false | Whether to include topic configuration in metadata (default `true`) |
| `schema_registry` | SchemaRegistryConfig | false | Schema Registry configuration |
| `tags` | TagsConfig | false | Tags to apply to discovered assets |

`tls`, `consumer_config` and the `authentication.type` and `authentication.mechanism` fields from the Kafka plugin are hidden because Confluent Cloud pins them to fixed values.

### AuthConfig

| Property | Type | Description |
|----------|------|-------------|
| `username` | string | Confluent Cloud API key |
| `password` | string | Confluent Cloud API secret (sensitive) |

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
