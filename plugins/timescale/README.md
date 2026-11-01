The TimescaleDB plugin discovers databases, tables, views, hypertables and continuous aggregates from TimescaleDB instances. It captures column information, foreign key relationships, view lineage, and the TimescaleDB detail a plain PostgreSQL read cannot see: partitioning dimensions, chunk counts, compression settings and background policies.

## Relationship to the PostgreSQL plugin

TimescaleDB is a PostgreSQL extension, so a TimescaleDB server is a PostgreSQL server. Assets are filed under the `PostgreSQL` provider by their bare object name, which is byte for byte the identity the PostgreSQL plugin produces. Running both plugins against the same instance updates one set of assets rather than creating two. This plugin is the PostgreSQL plugin plus hypertable knowledge.

Objects in the `_timescaledb_internal`, `_timescaledb_catalog`, `_timescaledb_config`, `_timescaledb_cache`, `timescaledb_information` and `timescaledb_experimental` schemas are skipped. They hold chunk tables, materialization hypertables and catalog views, and one busy hypertable puts thousands of chunks there.

## Required Permissions

The user needs read access to the catalog and to the tables being described:

```sql
GRANT CONNECT ON DATABASE your_db TO marmot_reader;
GRANT USAGE ON SCHEMA public TO marmot_reader;
GRANT SELECT ON ALL TABLES IN SCHEMA public TO marmot_reader;
```

## Querying

This plugin lets Marmot run read-only SQL against a TimescaleDB source for data queries. Every statement is planned with `EXPLAIN (FORMAT JSON, VERBOSE)` and then run inside a `READ ONLY` transaction over the extended query protocol, which also refuses more than one statement per call, so a statement can neither write nor smuggle a `COMMIT` to escape the transaction. The plan is walked for `ModifyTable` and `LockRows` nodes, which catches a data-modifying CTE hidden inside an otherwise read-only `SELECT`.

TimescaleDB plans a query over a hypertable as a scan of its chunk tables, and a query over a continuous aggregate as a scan of its hidden materialization hypertable's chunks. The plugin resolves these back to the hypertable or continuous aggregate discovery catalogued, using `timescaledb_information.chunks` and `timescaledb_information.continuous_aggregates`, so grants written against a catalog asset bind on the query path too.

None of this is a substitute for database-level permissions: the `READ ONLY` transaction stops a statement from writing, but the connecting user's own privileges are the real boundary on what it can read. Use a dedicated read-only role for `marmot_reader` rather than relying on Marmot alone.

`query_database` sets the database queries connect to. It defaults to whatever `database` is set to, or `postgres` when `database` is left empty and discovery reads every database on the server.

## Tests

Unit tests run anywhere. The end to end tests need a real server:

```
docker run -d --name timescale -p 15436:5432 \
  -e POSTGRES_PASSWORD=postgres -e POSTGRES_DB=shop \
  timescale/timescaledb:latest-pg16
docker exec -i timescale psql -U postgres -d shop < timescale/testdata/seed.sql

MARMOT_TEST_TIMESCALE_HOST=localhost MARMOT_TEST_TIMESCALE_PORT=15436 go test ./...
```
