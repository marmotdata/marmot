The CockroachDB plugin discovers databases, tables, views and materialized views from CockroachDB clusters. It captures columns (including computed column expressions and comments), row and column counts, table sizes, partitioning, foreign key relationships and, for each view, a lineage edge from every table it reads to the view.

Tables and views are named `database.schema.table`, since one cluster holds many databases. Every database except `system` is discovered by default; set `database` to discover one, or `exclude_databases` to skip more. Hidden columns CockroachDB adds on its own (the `rowid` of a table without a primary key, the shard column of a hash-sharded index) are left out.

## Connection

The plugin speaks the PostgreSQL wire protocol over the SQL port (26257 by default). Set `ssl_mode` to `verify-full` with `ssl_root_cert` for secure clusters, and `ssl_cert` plus `ssl_key` for certificate authentication. Leave `password` empty on insecure clusters.

## Required Permissions

The user needs to connect to each database and read its tables:

```sql
CREATE USER marmot_reader WITH PASSWORD 'your-password';
GRANT CONNECT ON DATABASE your_db TO marmot_reader;
GRANT USAGE ON SCHEMA your_db.public TO marmot_reader;
GRANT SELECT ON ALL TABLES IN SCHEMA your_db.public TO marmot_reader;
```

Table sizes come from span statistics, which need one extra system privilege. Without it the plugin logs a warning and skips `asset.size_bytes`:

```sql
GRANT SYSTEM VIEWACTIVITYREDACTED TO marmot_reader;
```
