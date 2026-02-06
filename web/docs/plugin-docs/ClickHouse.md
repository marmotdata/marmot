The ClickHouse plugin discovers databases, tables, and views from ClickHouse instances. It extracts schema information, column details, and table metrics like row counts and storage sizes.

## Connection Examples

import { Collapsible } from "@site/src/components/Collapsible";

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
