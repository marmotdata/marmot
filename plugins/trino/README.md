# marmot-plugin-trino

Marmot plugin for [Trino](https://trino.io/). Connects to a coordinator and discovers:

- **Catalogs** as `Catalog` assets (Trino-internal connectors like `memory` and `tpch` are skipped; `system` and `jmx` are excluded by default).
- **Tables and views** as `Table`/`View` assets with column schemas, table comments, and `SHOW CREATE TABLE` DDL. Assets are named and MRN'd to match the native plugin for their connector (a table behind the `postgresql` connector gets a PostgreSQL MRN), so they merge with assets discovered directly from the source system.
- **Lineage**: CONTAINS edges from each catalog to its tables and views.
- Optional **table statistics** (`include_stats`) via `SHOW STATS`.
- Optional **AI enrichment** via a Trino AI connector catalog: auto-generated descriptions for undocumented tables and table classification.

## Example Configuration

```yaml
host: "trino.company.com"
port: 8080
user: "marmot_reader"
secure: false
exclude_catalogs:
  - "system"
  - "jmx"
tags:
  - "trino"
  - "production"
```

Authentication supports plain user, password over HTTPS, and JWT bearer tokens (`access_token`). AI enrichment:

```yaml
ai_catalog: "llm"
ai_generate_descriptions: true
ai_classify_tables: true
ai_max_enrichments: 100
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
