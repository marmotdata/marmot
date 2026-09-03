The SQLite plugin discovers tables, views and foreign key relationships from SQLite database files.

It uses the pure-Go `modernc.org/sqlite` driver, so it needs no cgo and opens the database read-only. Turso and libSQL files are stored in the SQLite on-disk format, so a local copy of one works the same as any other SQLite file.

## File Sources

The `path` field accepts local paths, S3 URIs (`s3://bucket/key`) or Git URIs (`git::https://...`). For S3 and Git sources, the file is downloaded to a temporary directory before discovery and cleaned up afterwards.

See [File Sources](./Shared%20Configuration/File%20Sources.md) for the full list of supported backends, authentication options and configuration examples.
