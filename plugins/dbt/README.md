The DBT plugin ingests metadata from dbt (Data Build Tool) projects, including models, sources, seeds, and lineage relationships. It reads dbt's generated artifacts to understand your data transformation layer and how it connects to your warehouse.

## Prerequisites

Before Marmot can ingest your dbt project, you need to generate the artifact files in your project's `target/` directory.

:::warning[Required]
Generate `manifest.json` by running:
```bash
dbt compile
```
:::

:::tip[Recommended]
Generate `catalog.json` for column types and statistics:
```bash
dbt docs generate
```
:::

## File Sources

The `target_path` field accepts local paths, S3 URIs (`s3://bucket/prefix`) or Git URIs (`git::https://...`). For S3 and Git sources, the target directory is downloaded to a temporary location before discovery and cleaned up afterwards.

See [File Sources](./Shared%20Configuration/File%20Sources.md) for the full list of supported backends, authentication options and configuration examples.

## Naming

A model is named by the database and schema it is built into and its own name, for example `mrn://asset/model/dbt/analytics.marts.orders`. Set `zone` to tell two dbt projects apart.

The warehouse tables dbt reads or builds belong to the warehouse's own plugin, so they use its name: `<database>.<schema>.<table>`, or `<schema>.<table>` for MySQL, ClickHouse, SingleStore, Spark and Glue. Set `target_zone` to the Marmot zone of that warehouse, for example `prod-snowflake`, so those tables merge with the ones the warehouse plugin catalogues.
