The Doris plugin discovers databases, tables, views and async materialized views from Apache Doris clusters. It reads the table model, partitioning, distribution and column aggregation types from the DDL, and emits row counts, data sizes, view lineage and foreign key relationships.

It connects to a frontend over the MySQL protocol (port 9030 by default) with `github.com/go-sql-driver/mysql`.

## Required Permissions

The user needs to read the catalog and the definitions of views:

```sql
CREATE USER 'marmot_reader'@'%' IDENTIFIED BY 'your-password';
GRANT SELECT_PRIV, SHOW_VIEW_PRIV ON internal.*.* TO 'marmot_reader'@'%';
```

`SHOW_VIEW_PRIV` is what lets `SHOW CREATE VIEW` run; without it views are catalogued without their query or lineage.
