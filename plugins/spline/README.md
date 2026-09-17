The Spline plugin discovers Spark applications and their lineage from a [Spline](https://absaoss.github.io/spline/) server.

It reads the Spline consumer API and creates one Pipeline asset per Spark application name, not per run, so a job that runs nightly stays a single asset. Every run in the window becomes a run history event, including failures.

## Lineage

Spline records the URI Spark read from or wrote to. The plugin resolves those URIs to the assets other Marmot plugins publish, so a Spark job links to the real PostgreSQL table or S3 bucket rather than to a copy of it. Resolved schemes: `jdbc:` for PostgreSQL, MySQL, MariaDB, SQL Server, Oracle, Redshift and Snowflake, `s3`/`s3a`/`s3n`, `gs`, `abfs`/`abfss`, `hive` and `delta`. Anything else produces no edge.

The plugin creates no tables or buckets of its own. An edge only appears in Marmot once the plugin that owns the other end has catalogued it.

## Column Lineage

With `include_column_lineage` the plugin reads Spline's attribute lineage for every column of a run's output and stores it on the pipeline as JSON under the `column_lineage` metadata key, keyed by the MRN of the table produced. Marmot's lineage edges carry no column information, so it is recorded as metadata rather than on the edge.

## Host

`host` is the Spline REST gateway root. A `/consumer` or `/producer` suffix is stripped, so pasting either API's URL works.
