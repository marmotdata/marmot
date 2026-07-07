# marmot-plugin-duckdb

Marmot plugin for DuckDB. Discovers schemas, tables, views and foreign key relationships from DuckDB database files, optionally enriched with column information and table metrics.

Marmot plugins are standalone binaries that the Marmot host launches on demand via [go-plugin](https://github.com/hashicorp/go-plugin) and talks to over gRPC. It is built on the [Marmot plugin SDK](https://github.com/marmotdata/plugin-sdk).

## File Sources

The `path` field accepts local paths, S3 URIs (`s3://bucket/key`) or Git URIs (`git::https://...`). For S3 and Git sources, the file is downloaded to a temporary directory before discovery and cleaned up afterwards.

```yaml
# Local file
path: "/data/analytics.duckdb"

# S3 (uses the default AWS credential chain unless s3_source is set)
path: "s3://my-bucket/warehouse/analytics.duckdb"

# Git repository (subpath after //, ref as query parameter)
path: "git::https://github.com/org/repo//data/analytics.duckdb?ref=main"
```

## Example Configuration

```yaml
path: "/data/analytics.duckdb"
include_columns: true
enable_metrics: true
discover_foreign_keys: true
exclude_system_schemas: true
filter:
  include:
    - "^main\\..*"
  exclude:
    - ".*_temp$"
tags:
  - "duckdb"
  - "analytics"
```

## Configuration

The following configuration options are available:

| Property | Type | Required | Description |
|----------|------|----------|-------------|
| `path` | string | true | Path to the DuckDB database file (local path, s3://bucket/key or git::url) |
| `include_columns` | bool | false | Whether to include column information in table metadata (default `true`) |
| `enable_metrics` | bool | false | Whether to include table metrics (row counts and sizes) (default `true`) |
| `discover_foreign_keys` | bool | false | Whether to discover foreign key relationships (default `true`) |
| `exclude_system_schemas` | bool | false | Whether to exclude system schemas (information_schema, pg_catalog) (default `true`) |
| `source_type` | string | false | File source backend (auto-detected from path when empty) |
| `s3_source` | S3SourceConfig | false | S3 file source configuration |
| `git_source` | GitSourceConfig | false | Git repository file source configuration |
| `external_links` | []ExternalLink | false | External links to show on all assets |
| `filter` | Filter | false | Filter discovered assets by name (regex) |
| `tags` | TagsConfig | false | Tags to apply to discovered assets |

### S3SourceConfig

| Property | Type | Description |
|----------|------|-------------|
| `credentials.use_default` | bool | Use AWS credentials from environment or default profile (recommended) |
| `credentials.id` | string | AWS access key ID |
| `credentials.secret` | string | AWS secret access key (sensitive) |
| `credentials.token` | string | AWS session token (sensitive) |
| `credentials.profile` | string | AWS profile to use from shared credentials file |
| `credentials.role` | string | AWS IAM role ARN to assume |
| `credentials.role_external_id` | string | External ID for cross-account role assumption |
| `credentials.region` | string | AWS region for services |
| `credentials.endpoint` | string | Custom endpoint URL for AWS services |

### GitSourceConfig

| Property | Type | Description |
|----------|------|-------------|
| `url` | string | Git repository URL |
| `ref` | string | Branch, tag or commit to check out (default `main`) |
| `path` | string | Subdirectory within the repository |
| `token` | string | Personal access token for HTTPS auth (sensitive) |
| `ssh_key_path` | string | Path to SSH private key for SSH auth |

## Lineage

Foreign key relationships between tables are discovered as `FOREIGN_KEY` lineage edges from the referencing table to the referenced table.

## Available Metadata

Table and view assets carry the following fields:

| Field | Type | Description |
|-------|------|-------------|
| `path` | string | Path to the DuckDB database file |
| `schema` | string | Schema name |
| `table_name` | string | Table or view name |
| `object_type` | string | Object type (BASE TABLE, VIEW) |

With `include_columns` enabled, each asset's schema contains the column list (name, data type, nullability and default expression). With `enable_metrics` enabled, `asset.size_bytes` and `asset.column_count` statistics are collected per table.

## Development

This plugin requires cgo (the DuckDB engine is linked in), so a C toolchain must be available when building.

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
