The QuestDB plugin discovers tables, views and materialized views from QuestDB instances. It captures the designated timestamp, partitioning, WAL, deduplication and TTL settings of each table, QuestDB column types verbatim, row counts, sizes on disk and partition counts, and links views to the tables they read.

It connects over the PostgreSQL wire protocol (port 8812) with `github.com/jackc/pgx/v5` and reads QuestDB's own table functions (`tables()`, `table_columns()`, `table_partitions()`, `views()`, `materialized_views()`), since QuestDB's `pg_catalog` is minimal. QuestDB has a single database and no schemas, so no Database asset is created and objects are addressed by their bare name.

## Required Permissions

QuestDB open source has a single login, `admin` with password `quest` by default, and no per-table permissions. On QuestDB Enterprise, use a user that can SELECT from the tables to discover; see QuestDB's access control documentation for the exact grants.
