The PostgreSQL plugin discovers databases, schemas, and tables from PostgreSQL instances. It captures column information, table metrics, and foreign key relationships for lineage.
Tables and views are identified in their MRN by `database.schema.object`, for example `mrn://asset/table/postgresql/shop.public.orders`. Set `zone` to a short name such as `prod-eu` when you run two servers of the same kind, and it becomes part of every MRN. The name is one you choose, not a cloud availability zone.


## Required Permissions

The user needs read access to the information schema:

```sql
GRANT CONNECT ON DATABASE your_db TO marmot_reader;
GRANT USAGE ON SCHEMA public TO marmot_reader;
GRANT SELECT ON ALL TABLES IN SCHEMA public TO marmot_reader;
```
