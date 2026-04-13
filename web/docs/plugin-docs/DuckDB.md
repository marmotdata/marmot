The DuckDB plugin discovers schemas, tables, views and foreign key relationships from DuckDB database files.

## File Sources

The `path` field accepts local paths, S3 URIs (`s3://bucket/key`) or Git URIs (`git::https://...`). For S3 and Git sources, the file is downloaded to a temporary directory before discovery and cleaned up afterwards.

See [File Sources](./Shared%20Configuration/File%20Sources.md) for the full list of supported backends, authentication options and configuration examples.
