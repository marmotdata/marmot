# marmot-plugin-iceberg

Marmot plugin for [Apache Iceberg](https://iceberg.apache.org/) catalogs. Supports REST catalogs and AWS Glue Data Catalog, and discovers:

- **Namespaces** as `Namespace` assets with their properties.
- **Tables** as `Table` assets with table UUID, location, format version, snapshot count and current snapshot stats (records, data files, size), sort order, table properties, and column schemas.
- **Views** as `View` assets with schema, properties, and the view's SQL definition.
- **Lineage** edges from each namespace to the tables and views it contains.

## Example Configurations

### REST catalog

```yaml
uri: "http://localhost:8181"
warehouse: "my-warehouse"
credential: "client-id:client-secret"
tags:
  - "iceberg"
```

Use `token` for bearer token auth instead of `credential`, `prefix` for catalogs served under a path prefix, and `properties` for any additional catalog properties.

### AWS Glue catalog

```yaml
catalog_type: "glue"
credentials:
  region: "us-east-1"
glue_catalog_id: "123456789012" # optional, defaults to caller's account
```

Glue authentication uses the standard AWS credential chain: static keys, a shared profile, an assumed role, or the environment defaults.

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
