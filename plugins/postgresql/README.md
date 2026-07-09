# marmot-plugin-postgresql

Marmot plugin for [PostgreSQL](https://www.postgresql.org/). Connects to a server, enumerates its databases, and discovers:

- **Databases** as `Database` assets with owner, size, encoding, collation, and connection settings.
- **Tables, views, and materialized views** as `Table`/`View` assets with owner, estimated row counts, sizes, comments, and column schemas (types, nullability, primary keys, defaults).
- **Lineage**: CONTAINS edges from each database to its tables/views, and FOREIGN_KEY edges between tables.
- **Statistics** per table: row count, column count, and total size in bytes.

Tables and views can be previewed in Marmot. The plugin runs `SELECT * FROM <table> LIMIT 20` and returns the rows.

## Example Configuration

```yaml
host: "prod-postgres.company.com"
port: 5432
user: "marmot_reader"
password: "${POSTGRES_PASSWORD}"
ssl_mode: "require"
tags:
  - "postgres"
  - "production"
```

`ssl_mode` accepts `disable`, `require`, `verify-ca`, or `verify-full`.

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
