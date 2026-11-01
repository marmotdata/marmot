The CockroachDB plugin discovers databases, tables, views and materialized views from CockroachDB clusters. It captures columns (including computed column expressions and comments), row and column counts, table sizes, partitioning, foreign key relationships and, for each view, a lineage edge from every table it reads to the view.

Tables and views are named `database.schema.table`, since one cluster holds many databases. Every database except `system` is discovered by default; set `database` to discover one, or `exclude_databases` to skip more. Hidden columns CockroachDB adds on its own (the `rowid` of a table without a primary key, the shard column of a hash-sharded index) are left out.

## Connection

The plugin speaks the PostgreSQL wire protocol over the SQL port (26257 by default). Set `ssl_mode` to `verify-full` with `ssl_root_cert` for secure clusters, and `ssl_cert` plus `ssl_key` for certificate authentication. Leave `password` empty on insecure clusters.

## Querying

This plugin lets Marmot run read-only SQL against a CockroachDB source for data queries. A statement is first checked outright: anything that is not a `SELECT`, `WITH`, `VALUES` or `TABLE` statement is refused before it reaches CockroachDB at all. Every surviving statement is then planned with `EXPLAIN (VERBOSE)` and, separately, run inside a `BEGIN TRANSACTION READ ONLY` over the extended query protocol, which also refuses more than one statement per call, so a statement can neither write nor smuggle a second statement past a semicolon. CockroachDB auto-commits an explicit transaction before running DDL inside it by default (`autocommit_before_ddl`), which would otherwise let a `CREATE TABLE` or similar slip past a read-only transaction unharmed; the plugin's query connections turn that setting off, so DDL stays inside the transaction and is refused there too. The plan is also walked for a nested write, which catches a data-modifying CTE or a square-bracket subquery, such as `SELECT * FROM [INSERT INTO t ... RETURNING ...]`, hidden inside an otherwise read-only statement.

`EXPLAIN (VERBOSE)` reports the tables a statement scans without a schema qualifier, so the plugin resolves each one against the connected database's own catalog, the same tables discovery lists, to land on the same MRN. A table name that exists in more than one schema of that database, or that belongs to a different database CockroachDB let the statement read directly, cannot be told apart this way; the plan is then marked incomplete and the statement needs a grant on the whole source instead. A statement that calls a user-defined function is treated the same way, since the function may run a query of its own that the plan never shows.

A statement that calls `set_config`, `setval`, `setseed` or a handful of `crdb_internal` functions such as `set_vmodule` is refused outright rather than merely marked incomplete, because on CockroachDB a plain `SET` or `set_config(..., is_local=false)` is not always undone when the transaction that ran it is rolled back, unlike on PostgreSQL. Several sources can share one pooled connection, so as a second layer the plugin also scrubs a connection's session state with `RESET ALL` before handing it to another caller, in case a future CockroachDB builtin has the same effect and is not yet in that refusal list.

None of this is a substitute for database-level permissions: the read-only transaction stops a statement from writing, but the connecting user's own privileges are the real boundary on what it can read. Use a dedicated role that only has `SELECT` on the schemas it should query, rather than relying on Marmot alone:

```sql
CREATE USER marmot_reader WITH PASSWORD 'your-password';
GRANT CONNECT ON DATABASE your_db TO marmot_reader;
GRANT USAGE ON SCHEMA your_db.public TO marmot_reader;
GRANT SELECT ON ALL TABLES IN SCHEMA your_db.public TO marmot_reader;
```

`query_database` sets the database queries connect to, since a connection reads one database at a time even though a statement can still name another database's table directly. It defaults to `database` when discovery is scoped to one database, or to `defaultdb` when discovery spans the whole cluster.

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
