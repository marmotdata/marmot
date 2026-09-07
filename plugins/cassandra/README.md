The Cassandra plugin discovers keyspaces, tables and materialized views from Apache Cassandra clusters. It reads the `system_schema` tables over CQL with the pure-Go `gocql` driver, so it works with Cassandra 3.0 and later and needs no cgo.

Tables and views are named `keyspace.table`, the same identity the OpenMetadata plugin projects for a Cassandra service, so the two routes land on one asset.

## Required Permissions

The user needs to read the schema tables, plus the keyspaces it should preview sample data from:

```sql
CREATE ROLE marmot_reader WITH PASSWORD = 'your-password' AND LOGIN = true;
GRANT SELECT ON KEYSPACE system_schema TO marmot_reader;
GRANT SELECT ON KEYSPACE your_keyspace TO marmot_reader;
```

## Statistics

Column and index counts are reported per table. A row count is not: Cassandra keeps no row estimate and `COUNT(*)` is a full cluster scan.

## Known Gaps

DataStax Astra secure connect bundles are not supported. Connect with `hosts` and `ssl` instead.
