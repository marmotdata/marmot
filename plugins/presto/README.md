The Presto plugin discovers catalogs, schemas, tables and views from Presto clusters.

Tables of connectors that another Marmot plugin covers (PostgreSQL, MySQL, Hive, Iceberg and the rest of the Trino connector map) are catalogued under that plugin's provider and name shape, so the same table reached through Presto and directly lands on one asset. Everything else, including memory and tpch tables, belongs to Presto under its full `catalog.schema.table` path.

## Required Permissions

The connecting user needs `SELECT` access to `system.metadata.catalogs`, `system.runtime.nodes` and each catalog's `information_schema`. `include_stats` additionally runs `SHOW STATS` per table. A password is only sent over HTTPS, so `secure: true` is required with one.
