The Superset plugin discovers dashboards, charts, datasets and database connections from Apache Superset through its REST API.

Dashboards contain their charts, datasets feed the charts that read them, and a dataset is linked to the table it reads in the database Superset connects to, using the identity that database's own Marmot plugin gives the table. Virtual datasets are linked to the tables named after FROM and JOIN in their SQL.

## Required Permissions

Log in with a user that can read dashboards, charts, datasets and databases. The built-in `Gamma` role is enough for dashboards, charts and datasets; reading database connection details (`GET /api/v1/database/{id}/connection`) needs `can_read` on `Database`, which `Alpha` and `Admin` have.

## Table lineage

A dataset is linked to a table only for backends Marmot has a naming rule for: PostgreSQL, MySQL, MariaDB, BigQuery, ClickHouse, SQLite, DuckDB, Athena (as Glue), Snowflake, Redshift, SQL Server, CockroachDB, Oracle, Druid, Trino and Databricks. The edge appears in Marmot once that database has been ingested with its own plugin.
