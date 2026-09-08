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
