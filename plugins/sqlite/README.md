The SQLite plugin discovers tables, views and foreign key relationships from SQLite database files.

It uses the pure-Go `modernc.org/sqlite` driver, so it needs no cgo and opens the database read-only. Turso and libSQL files are stored in the SQLite on-disk format, so a local copy of one works the same as any other SQLite file.

## File Sources

The `path` field accepts local paths, S3 URIs (`s3://bucket/key`) or Git URIs (`git::https://...`). For S3 and Git sources, the file is downloaded to a temporary directory before discovery and cleaned up afterwards.

See [File Sources](./Shared%20Configuration/File%20Sources.md) for the full list of supported backends, authentication options and configuration examples.

## Querying

This plugin lets Marmot run read-only SQL against a SQLite file for data queries. Every statement is checked before it reaches SQLite: anything other than a single `SELECT`, `WITH`, `VALUES` or `TABLE` statement is refused, along with `ATTACH`, `DETACH`, `PRAGMA`, `VACUUM`, a call to `load_extension` and a second statement hidden after a semicolon. The connection itself is opened with `mode=ro` and `PRAGMA query_only=ON`, which is the real boundary: SQLite refuses to write to the file at all, even if a check above were somehow bypassed.

Planning uses `EXPLAIN` to read a statement's compiled bytecode rather than running it, and refuses anything whose bytecode would write, which catches a write hidden behind a leading `WITH`. The tables a statement's bytecode opens for reading are mapped back to the same MRN discovery gives them, using `sqlite_master`, so grants written against catalogued assets bind on the query path too. SQLite always reads a view by reading its base tables rather than the view itself, so a query over a view is reported and granted against those base tables.

A statement that calls a table-valued function such as `pragma_table_info` or `json_each` can read a table the bytecode never names, so its plan is marked incomplete and needs a grant on the whole source.
