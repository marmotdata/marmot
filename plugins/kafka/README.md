Marmot plugin for Apache Kafka. Discovers topics from Kafka clusters, captures topic configurations and partition details and optionally enriches assets with schemas from a Confluent Schema Registry.

Marmot plugins are standalone binaries that the Marmot host launches on demand via [go-plugin](https://github.com/hashicorp/go-plugin) and talks to over gRPC. It is built on the [Marmot plugin SDK](https://github.com/marmotdata/plugin-sdk).

> Looking for a managed service? Marmot has dedicated plugins for [Confluent Cloud](./Confluent%20Cloud) and [Redpanda](./Redpanda) with pre-configured defaults.

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
