The Dagster plugin discovers jobs, the ops inside them, and software-defined assets from a Dagster webserver.

It reads everything through the GraphQL endpoint at `{host}/graphql`, so it works against both open source Dagster and Dagster+. For Dagster+, set `token` and the plugin sends it as the `Dagster-Cloud-Api-Token` header.

## Assets

- `Pipeline` per job, named after the job. Schedules, sensors and a summary of recent runs are attached as metadata.
- `Task` per op, named `<job>/<op handle>`.
- `Dataset` per software-defined asset, named after its asset key joined with `/`. A `TableSchema` metadata entry becomes the asset's columns.

Dagster's implicit asset jobs (the ones named `__ASSET_JOB`) are skipped.

## Lineage

Pipelines contain their Tasks, Tasks depend on the Tasks feeding their inputs, Datasets feed the Datasets built from them, and a Pipeline produces every Dataset it materialises. When an asset names a warehouse table and its compute kind is Snowflake, BigQuery, Postgres or DuckDB, the Dataset also produces that native table.
