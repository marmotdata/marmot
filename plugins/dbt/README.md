# marmot-plugin-dbt

Marmot plugin for [DBT (Data Build Tool)](https://www.getdbt.com/). Reads a DBT project's `target/` artifacts (`manifest.json`, `catalog.json`, optionally `run_results.json`) and produces:

- **Models** as `Model` assets plus their materialized `Table`/`View`/`Materialized View`/etc. depending on the target adapter.
- **Sources** and **seeds** as `Table` assets.
- **Lineage** edges between models, sources and seeds (using `depends_on` from the manifest).

Adapters for Snowflake, BigQuery, Redshift, Postgres, MySQL, Databricks, DuckDB, ClickHouse, Trino, Athena, Glue, Materialize, Oracle, SQL Server and more map dbt materializations to the correct asset type and provider.

## Example Configurations

### Local target directory

```yaml
target_path: "/path/to/dbt/project/target"
project_name: "analytics"
environment: "production"
```

### S3-hosted artifacts

```yaml
target_path: "s3://my-dbt-artifacts/analytics/target/"
project_name: "analytics"
environment: "production"
s3_source:
  credentials:
    region: "eu-west-2"
    use_default: true
```

### Git repository

```yaml
target_path: "git::https://github.com/example/analytics-dbt//target?ref=main"
project_name: "analytics"
environment: "production"
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
