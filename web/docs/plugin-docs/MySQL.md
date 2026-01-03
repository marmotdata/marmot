The MySQL plugin discovers databases and tables from MySQL instances. It captures column information, row counts, and foreign key relationships for lineage.

## Required Permissions

The user needs read access to the information schema:

```sql
CREATE USER 'marmot_reader'@'%' IDENTIFIED BY 'your-password';
GRANT SELECT ON your_database.* TO 'marmot_reader'@'%';
GRANT SELECT ON information_schema.* TO 'marmot_reader'@'%';
```
