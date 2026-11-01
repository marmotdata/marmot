The DuckDB plugin discovers schemas, tables, views and foreign key relationships from DuckDB database files.

## File Sources

The `path` field accepts local paths, S3 URIs (`s3://bucket/key`) or Git URIs (`git::https://...`). For S3 and Git sources, the file is downloaded to a temporary directory before discovery and cleaned up afterwards.

See [File Sources](./Shared%20Configuration/File%20Sources.md) for the full list of supported backends, authentication options and configuration examples.

## Querying

The plugin supports Marmot's data queries. A statement is planned and run against a connection opened with `access_mode=read_only`, which DuckDB enforces at the engine level: no statement on that connection can write, regardless of how it is phrased. `enable_external_access=false` additionally blocks anything that would reach outside the file itself, such as `read_csv`, `read_parquet`, `glob`, `ATTACH`, `COPY ... TO` and installing or loading an extension; `lock_configuration=true` stops a statement from turning any of that back on with its own `SET`. Only a single statement is accepted per call.

This lockdown governs what a statement running through Marmot can do; it is not a substitute for file permissions. The account the Marmot host runs as still needs read access to the underlying file (and nothing more), and that remains the outer boundary this plugin depends on.
