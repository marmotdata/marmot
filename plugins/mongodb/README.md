# marmot-plugin-mongodb

Marmot plugin for [MongoDB](https://www.mongodb.com/). Discovers:

- **Databases** as `Database` assets with size, collection/view/index counts (system databases excluded by default).
- **Collections** as `Collection` assets with size, document count, capped/sharding status, storage engine, and validation settings.
- **Views** as `View` assets with the source collection and aggregation pipeline.
- **Lineage**: CONTAINS edges from each database to its collections, and VIEW_OF edges from source collections to views.

Connect with a full connection URI or host/port plus credentials; TLS is supported.

## Example Configurations

### Host and credentials

```yaml
host: "mongo-cluster.company.com"
port: 27017
user: "analytics_reader"
password: "${MONGO_PASSWORD}"
auth_source: "admin"
tls: true
tags:
  - "mongodb"
  - "analytics"
```

### Connection URI

```yaml
connection_uri: "mongodb+srv://analytics_reader:${MONGO_PASSWORD}@cluster0.example.mongodb.net/"
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
