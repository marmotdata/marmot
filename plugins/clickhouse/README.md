The ClickHouse plugin discovers databases, tables, and views from ClickHouse instances. It extracts schema information, column details, and table metrics like row counts and storage sizes.

## Connection Examples

<Collapsible title="Basic Connection" icon="simple-icons:clickhouse">

```yaml
host: "clickhouse.company.com"
port: 9000
user: "default"
password: "${CLICKHOUSE_PASSWORD}"
database: "default"
include_databases: true
include_columns: true
enable_metrics: true
tags:
  - "clickhouse"
  - "analytics"
```

</Collapsible>

<Collapsible title="ClickHouse Cloud" icon="mdi:cloud">

```yaml
host: "your-instance.clickhouse.cloud"
port: 9440
user: "default"
password: "${CLICKHOUSE_PASSWORD}"
secure: true
include_databases: true
include_columns: true
enable_metrics: true
filter:
  include:
    - "^analytics.*"
  exclude:
    - ".*_temp$"
tags:
  - "clickhouse"
  - "cloud"
```

</Collapsible>

## Querying

This plugin lets Marmot run read-only SQL against a ClickHouse source for data queries. A statement's first keyword is checked outright: anything other than `SELECT` or `WITH` is refused before it reaches ClickHouse at all, and so is a statement that calls a table function reading data from outside ClickHouse itself (`url`, `s3`, `file`, `remote`, `remoteSecure`, `cluster`, `mysql`, `postgresql`, `odbc`, `jdbc`, `hdfs`, `azureBlobStorage` or `input`), since planning must never risk dialling out. Every surviving statement runs with the `readonly` setting, which ClickHouse enforces server-side by refusing writes, DDL and changes to settings; the native protocol also accepts one statement per call, so a statement can neither write nor smuggle a second one past a semicolon. `readonly` alone does not stop the table functions above, which is why they are refused outright rather than left to it.

Each statement is also planned with `EXPLAIN PLAN`, so ClickHouse does the SQL parsing, and every `ReadFromMergeTree` node in the plan is resolved to the same `database.table` MRN discovery gives that table. Views, normal and materialised, are expanded by the planner into the tables behind them, so a statement over a view names those tables rather than the view itself. A read the plan cannot name as a table this way, such as a non-MergeTree engine or a call to the `dictGet` family reading a dictionary, marks the plan incomplete instead, and the statement then needs a grant on the whole source.

None of this is a substitute for database-level permissions: `readonly` stops a statement from writing, but the connecting user's own grants are the real boundary on what it can read. Use a dedicated user with the built-in `readonly` settings profile and `SELECT` grants only on the databases it should query, rather than relying on Marmot alone:

```sql
CREATE USER marmot_reader IDENTIFIED WITH plaintext_password BY 'your-password' SETTINGS PROFILE 'readonly';
GRANT SELECT ON your_database.* TO marmot_reader;
```

## Required Permissions

The user needs read access to system tables:

```sql
GRANT SELECT ON system.databases TO marmot_user;
GRANT SELECT ON system.tables TO marmot_user;
GRANT SELECT ON system.columns TO marmot_user;
```

For read-only discovery of all databases:

```sql
GRANT SHOW DATABASES ON *.* TO marmot_user;
GRANT SHOW TABLES ON *.* TO marmot_user;
GRANT SHOW COLUMNS ON *.* TO marmot_user;
```
