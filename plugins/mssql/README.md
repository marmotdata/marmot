The SQL Server plugin discovers databases, tables, views, stored procedures and functions from Microsoft SQL Server, Azure SQL Database and Azure SQL Edge instances.

It uses the pure-Go `github.com/microsoft/go-mssqldb` driver, so it needs no cgo and no ODBC driver on the host. SQL authentication works with a plain login, and Windows authentication works by giving the login as `DOMAIN\user`.

## Databases

A SQL Server session is bound to a single database, so the plugin opens one connection per database. By default it discovers every database the login can open, minus `master`, `model`, `msdb` and `tempdb`. Set `database` to discover just one.

Because one instance holds many databases and two of them can hold the same schema and object name, tables, views and routines are named `database.schema.object`.

## Encryption

`encrypt: true` requires an encrypted connection. A default SQL Server install presents a self-signed certificate, which fails verification, so pair it with `trust_server_certificate: true` or install a certificate the client trusts. `encrypt: false` turns encryption off entirely.

## Comments

SQL Server has no `COMMENT ON`, so descriptions come from `MS_Description` extended properties on schemas, tables, views and columns. Objects without one have no description.

## Query history

Query history is not read. Usage-based lineage would need the plan cache or Query Store, which are per-database, expensive to scan and often disabled. Lineage comes from foreign keys and view definitions instead.
