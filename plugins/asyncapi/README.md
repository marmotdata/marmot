# marmot-plugin-asyncapi

Marmot plugin for [AsyncAPI](https://www.asyncapi.com/) v3 specifications. Walks a spec file or directory of specs and produces:

- **Services** as `Service` assets from the spec's `info` section (contact, license, servers, protocols).
- **Channels** as protocol-specific assets based on channel bindings: Kafka topics, AMQP exchanges/queues, SNS topics, SQS queues, Google Pub/Sub topics, MQTT topics, NATS subjects, Pulsar topics, Solace queues/topics, IBM MQ queues/topics, JMS destinations, WebSocket channels, Anypoint MQ destinations and HTTP endpoints. Channels without bindings become generic `Channel` assets.
- **Message schemas** attached to channel assets (payload and headers).
- **Lineage** edges between services and channels from operations (`send` → PRODUCES, `receive` → CONSUMES).

Only AsyncAPI v3 documents are processed; older versions are skipped.

## Example Configurations

### Local spec directory

```yaml
spec_path: "/app/asyncapi-specs"
environment: "production"
discover_services: true
discover_channels: true
discover_messages: true
tags:
  - "asyncapi"
  - "event-driven"
```

### S3-hosted specs

```yaml
spec_path: "s3://my-specs-bucket/asyncapi/"
environment: "production"
s3_source:
  credentials:
    region: "eu-west-2"
    use_default: true
```

### Git repository

```yaml
spec_path: "git::https://github.com/example/event-specs//asyncapi?ref=main"
environment: "production"
git_source:
  token: "${GITHUB_TOKEN}"
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
