The Hive plugin discovers databases, tables, views and materialized views from Apache Hive through HiveServer2. It captures columns with their Hive types and comments, partition and bucket layout, storage formats, table statistics, view queries and foreign key constraints for lineage.

It speaks the HiveServer2 Thrift protocol with the pure-Go `github.com/beltran/gohive` driver over the binary or HTTP transport, with NONE, NOSASL, LDAP or KERBEROS authentication and optional TLS.

## Required Permissions

The user needs `SELECT` on the databases to discover. Discovery runs `SHOW DATABASES`, `DESCRIBE DATABASE EXTENDED`, `SHOW TABLES`, `SHOW VIEWS`, `SHOW MATERIALIZED VIEWS`, `DESCRIBE FORMATTED`, `SHOW PARTITIONS` and, with `include_ddl`, `SHOW CREATE TABLE`. Sample data previews run `SELECT * ... LIMIT 20`.

## Notes

- Row counts and sizes come from the metastore statistics (`numRows`, `totalSize`), which are as fresh as the last `ANALYZE TABLE` or stats-collecting write.
- Kerberos needs a build with the driver's `kerberos` tag and the GSSAPI C library: `go build -tags kerberos .`. The published binaries are built without it and report an error when `auth: KERBEROS` is used.
- Hive 4 creates non-ACID tables as external tables (`TRANSLATED_TO_EXTERNAL`), so `object_type` reports what Hive itself reports.
