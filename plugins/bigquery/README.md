# marmot-plugin-bigquery

Marmot plugin for [Google BigQuery](https://cloud.google.com/bigquery). Discovers the datasets and tables in a project and produces:

- **Datasets** as `Dataset` assets with location, timestamps, expiration defaults, labels, and access-entry counts.
- **Tables, views, and external tables** as `Table`/`View`/`ExternalTable` assets with partitioning, clustering, labels, optional row/size statistics, view queries, external data configuration, and a JSON Schema derived from the table schema.
- **Lineage** edges from each dataset to the tables it contains.

Tables and views can be previewed in Marmot. The plugin runs `SELECT * FROM <table> LIMIT 20` and returns the rows.

Authentication is one of a service account credentials file, inline credentials JSON, or application default credentials. Set `BIGQUERY_EMULATOR_HOST` to run against a BigQuery emulator.

## Example Configurations

### Service account credentials file

```yaml
project_id: "company-data-warehouse"
credentials_path: "/etc/marmot/bq-service-account.json"
tags:
  - "bigquery"
  - "data-warehouse"
```

### Application default credentials

```yaml
project_id: "company-data-warehouse"
use_default_credentials: true
exclude_system_datasets: true
include_table_stats: true
```

### Inline credentials

```yaml
project_id: "company-data-warehouse"
credentials_json: "${BIGQUERY_CREDENTIALS_JSON}"
filter:
  include:
    - "^analytics_.*"
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
