The PostgreSQL plugin discovers databases, schemas, and tables from PostgreSQL instances. It captures column information, table metrics, and foreign key relationships for lineage.

## Required Permissions

The user needs read access to the information schema:

```sql
GRANT CONNECT ON DATABASE your_db TO marmot_reader;
GRANT USAGE ON SCHEMA public TO marmot_reader;
GRANT SELECT ON ALL TABLES IN SCHEMA public TO marmot_reader;
```
