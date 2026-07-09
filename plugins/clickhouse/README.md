# marmot-plugin-clickhouse

Marmot plugin for [ClickHouse](https://clickhouse.com/). Connects over the native protocol and discovers:

- **Databases** as `Database` assets (system databases excluded).
- **Tables and views** as `Table`/`View` assets with engine, row counts, sizes, comments, column schemas, and the `CREATE TABLE` query.
- **Lineage** edges from each database to the tables it contains.
- **Statistics** per table: row count, size in bytes, and column count (when `enable_metrics` is on).

Tables and views can be previewed in Marmot. The plugin runs `SELECT * FROM <table> LIMIT 20` and returns the rows.

## Example Configuration

```yaml
host: "clickhouse.company.com"
port: 9000
user: "default"
password: "${CLICKHOUSE_PASSWORD}"
database: "default"
secure: false
include_databases: true
include_columns: true
enable_metrics: true
exclude_system_tables: true
filter:
  include:
    - "^analytics.*"
  exclude:
    - ".*_temp$"
tags:
  - "clickhouse"
  - "analytics"
```

Set `secure: true` for TLS connections. `port` is the native protocol port (9000 by default, 9440 for TLS on ClickHouse Cloud).

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
