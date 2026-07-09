# marmot-plugin-redis

Marmot plugin for [Redis](https://redis.io/). Reads `INFO` from a server and produces a `Database` asset per keyspace database (db0-db15) with key counts, expiring-key counts, average TTL, and server context (version, role, memory usage, eviction policy, connected clients).

Set `discover_all_databases: false` to only produce the database configured in `db`.

## Example Configuration

```yaml
host: "redis.company.com"
port: 6379
password: "${REDIS_PASSWORD}"
discover_all_databases: true
filter:
  include:
    - "^db[0-3]$"
tags:
  - "redis"
  - "cache"
```

Use `username` for ACL auth, and `tls`/`tls_insecure` for TLS connections.

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
