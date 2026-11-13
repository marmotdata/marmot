The Presto plugin discovers catalogs, schemas, tables and views from Presto clusters.

Tables are Presto's own assets by default, named `<catalog>.<schema>.<table>` under the `Presto` provider, for example `mrn://asset/table/presto/memory.shop.orders`. A catalog's connector says what kind of system it reads but not which server, so the plugin never guesses a native identity: tables of a catalog that is not in `catalog_map` stay Presto's, and a warning in the log names each such catalog because its tables won't merge with the native plugin's.

To make a catalog's tables the same assets another Marmot plugin discovers (PostgreSQL, MySQL, Hive, Iceberg and the rest of the connector map), map it to the zone of that plugin and, for connectors whose native names start with a database, to the database:

```yaml
catalog_map:
  warehouse_pg:
    zone: prod-eu      # the zone option of the PostgreSQL plugin
    database: shop     # the database the catalog points at
```

A mapped table gets the native plugin's MRN, for example `mrn://asset/table/postgresql@prod-eu/shop.public.orders`. `database` is required for PostgreSQL, SQL Server, Oracle, Redshift and Snowflake (the database) and BigQuery (the project). Memory and tpch tables always belong to Presto. Set `zone` on the plugin itself to tell two Presto clusters apart (a name you choose, not a cloud availability zone).

## Required Permissions

The connecting user needs `SELECT` access to `system.metadata.catalogs`, `system.runtime.nodes` and each catalog's `information_schema`. `include_stats` additionally runs `SHOW STATS` per table. A password is only sent over HTTPS, so `secure: true` is required with one.
