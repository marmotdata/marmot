# marmot-plugin-deltalake

Marmot plugin for [Delta Lake](https://delta.io/) tables. Reads the `_delta_log` transaction log of each configured table directory (JSON commits plus Parquet checkpoints) and produces a `Table` asset per table with its current version, active file count and total size, format, partition columns, table properties, protocol versions, and the column schema.

Table paths can be local directories, `s3://bucket/prefix`, or `git::` URLs; remote paths are downloaded to a temp directory for reading.

## Example Configurations

### Local tables

```yaml
table_paths:
  - "/data/delta/events"
  - "/data/delta/users"
tags:
  - "delta-lake"
```

### S3-hosted tables

```yaml
table_paths:
  - "s3://lake-bucket/delta/events"
s3_source:
  credentials:
    region: "eu-west-2"
    use_default: true
```

### Git repository

```yaml
table_paths:
  - "git::https://github.com/example/lake//tables/events?ref=main"
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
