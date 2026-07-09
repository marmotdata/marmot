# marmot-plugin-mysql

Marmot plugin for [MySQL](https://www.mysql.com/). Connects to a database and discovers:

- **Tables and views** as `Table`/`View` assets with engine, estimated row counts, data and index sizes, collation, timestamps, and comments.
- **Lineage**: FOREIGN_KEY edges between tables derived from referential constraints.

Tables and views can be previewed in Marmot. The plugin runs `SELECT * FROM <table> LIMIT 20` and returns the rows.

## Example Configuration

```yaml
host: "mysql-prod.internal"
port: 3306
user: "marmot_user"
password: "${MYSQL_PASSWORD}"
database: "ecommerce"
tls: "true"
tags:
  - "mysql"
  - "ecommerce"
```

`tls` accepts `false`, `true`, `skip-verify`, or `preferred`.

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
