# marmot-plugin-nats

Marmot plugin for [NATS](https://nats.io/) JetStream. Lists the streams on a server and produces a `Stream` asset per stream with its subjects, retention policy, limits (bytes, messages, age, message size), storage type, replica count, and current state (message count, size, consumer count, sequence numbers).

Authentication supports tokens, username/password, and NATS credentials files; TLS is supported.

## Example Configuration

```yaml
host: "nats.company.com"
port: 4222
token: "${NATS_TOKEN}"
filter:
  include:
    - "^ORDERS"
tags:
  - "nats"
  - "messaging"
```

Use `username`/`password` or `credentials_file` instead of `token` for those auth schemes.

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
