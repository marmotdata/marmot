The MariaDB plugin discovers a database and its tables, views and sequences from a MariaDB server. It captures column information (including invisible and generated columns), row and size statistics, foreign key relationships and view lineage (a view is fed by the tables it reads: base table -> view).

## Required Permissions

The user needs read access to the database and the information schema. `SHOW VIEW` is needed to read view definitions:

```sql
CREATE USER 'marmot_reader'@'%' IDENTIFIED BY 'your-password';
GRANT SELECT, SHOW VIEW ON your_database.* TO 'marmot_reader'@'%';
```
