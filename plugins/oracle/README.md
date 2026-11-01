The Oracle plugin discovers schemas, tables, views, materialized views and stored procedures from Oracle databases. It captures column information, primary and foreign keys, optimizer statistics and view lineage.

It uses the pure-Go `github.com/sijms/go-ora/v2` driver, so it needs no Oracle Instant Client.

## Naming

Oracle users treat a schema as the database, so each schema becomes a `Database` asset named after the schema (`HR`). Tables, views and procedures are named `SCHEMA.OBJECT` (`HR.EMPLOYEES`), the same shape the OpenMetadata and Trino plugins use for Oracle, so the three land on one asset. Identifiers keep the case Oracle stores them in, which is upper case unless they were created quoted.

Sequences are not discovered.

## Lineage

- `CONTAINS` from each schema's `Database` asset to its tables, views and procedures.
- `FOREIGN_KEY` from the referencing table to the referenced table, including references into another discovered schema.
- `VIEW_OF` from each base table to the view or materialized view that reads it, extracted from the view's SQL.

## Required Permissions

The user needs to read the data dictionary and the objects it should catalogue:

```sql
CREATE USER marmot_reader IDENTIFIED BY "your-password";
GRANT CREATE SESSION TO marmot_reader;
GRANT SELECT ANY TABLE TO marmot_reader;
```

`ALL_*` dictionary views only list objects the user can access. Procedures, functions and packages need `EXECUTE ANY PROCEDURE` (or an `EXECUTE` grant per object) to show up. Row counts and sizes come from optimizer statistics, so run `DBMS_STATS.GATHER_SCHEMA_STATS` for schemas that have never been analyzed.

With `use_dba_views: true` the plugin reads `DBA_*` views instead, which list every object regardless of grants. That needs `SELECT ANY DICTIONARY` (or `SELECT_CATALOG_ROLE`):

```sql
GRANT SELECT ANY DICTIONARY TO marmot_reader;
```

Sample data previews read the table directly, so they need `SELECT` on it.

## Querying

The plugin lets Marmot run read-only SQL against the source. Only a single `SELECT` or `WITH` statement is accepted; PL/SQL blocks, `CALL`, `EXEC`, `ALTER SESSION`, `LOCK TABLE`, `SELECT ... FOR UPDATE`, every DML and DDL statement, a `WITH` clause that declares an inline `FUNCTION` or `PROCEDURE`, and a trailing semicolon or a second statement are all refused before anything reaches Oracle. The statement that gets through still runs inside a transaction that issues Oracle's own `SET TRANSACTION READ ONLY` as its first statement and is always rolled back afterwards, never committed. Oracle's `BeginTx` has no read-only transaction option of its own, and DDL auto-commits and ends a transaction regardless of the mode it started in, which is why statements are refused up front rather than relied on to fail inside the transaction.

Oracle gives no side-effect-free way to ask what a statement would read: `EXPLAIN PLAN FOR` writes its result to `PLAN_TABLE`, and `V$SQL_PLAN` only knows a statement after it has already run. The plugin therefore never plans a statement at the table level, so Marmot falls back to requiring a grant on the whole source for Oracle queries, rather than on the individual tables a statement touches. Because of that, the Oracle user the source connects as is the access boundary that actually matters: it should be granted only `SELECT` on the schemas and objects callers are meant to query through Marmot, for example with a dedicated read-only user rather than one holding `SELECT ANY TABLE`.

A stored function marked `PRAGMA AUTONOMOUS_TRANSACTION` can write and commit even when called from a plain `SELECT`, because its transaction is independent of the read-only one the statement runs in; there is no way to tell from the statement text alone whether a function it calls does this, so it cannot be refused lexically the way an inline `WITH` clause function can. The connecting user must not hold `EXECUTE` on any function or package that writes, for the same reason it should not hold write grants on tables: the account's own privileges, not the query gate, are what actually stop a write.
