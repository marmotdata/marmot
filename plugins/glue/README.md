# marmot-plugin-glue

Marmot plugin for [AWS Glue](https://aws.amazon.com/glue/). Discovers:

- **Jobs** as `Job` assets with role, worker configuration, script location, connections, and timestamps.
- **Databases** as `Database` assets with catalog ID, location, and parameters.
- **Tables** as `Table` assets with storage descriptor details (location, formats, serde), partition keys, and column schemas. Iceberg tables are skipped; use the iceberg plugin for those.
- **Crawlers** as `Crawler` assets with targets, schedule, schema change policy, and last crawl status.

Authentication uses the standard AWS credential chain: static keys, a shared profile, an assumed role, or the environment defaults.

## Example Configuration

```yaml
credentials:
  region: "us-east-1"
  profile: "production"
  role: "arn:aws:iam::123456789012:role/MarmotDiscovery"
tags:
  - "aws"
discover_jobs: true
discover_databases: true
discover_tables: true
discover_crawlers: true
```

Each discovery type can be turned off individually; all four default to on.

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
