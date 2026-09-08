---
title: Oracle
description: Discovers schemas, tables, views, materialized views and stored procedures from Oracle databases.
status: experimental
---

# Oracle

<div class="flex flex-col gap-3 mb-6 pb-6 border-b border-gray-200">
<div class="flex items-center gap-3">
<span class="inline-flex items-center rounded-full px-4 py-2 text-sm font-medium bg-earthy-yellow-300 text-earthy-yellow-900">Experimental</span>
</div>
<div class="flex items-center gap-2">
<span class="text-sm text-gray-500">Creates:</span>
<div class="flex flex-wrap gap-2"><span class="inline-flex items-center rounded-lg px-4 py-2 text-sm font-medium bg-earthy-green-100 text-earthy-green-800 border border-earthy-green-300">Assets</span><span class="inline-flex items-center rounded-lg px-4 py-2 text-sm font-medium bg-earthy-green-100 text-earthy-green-800 border border-earthy-green-300">Lineage</span></div>
</div>
</div>

import { CalloutCard } from '@site/src/components/DocCard';

<CalloutCard
  title="Configure in the UI"
  description="This plugin can be configured directly in the Marmot UI with a step-by-step wizard."
  docId="Populating/UI"
  buttonText="View Guide"
  variant="secondary"
  icon="mdi:cursor-default-click"
/>


The Oracle plugin discovers schemas, tables, views, materialized views and stored procedures from Oracle databases. It reads the data dictionary (`ALL_*` views, or `DBA_*` with `use_dba_views`) for columns, primary and foreign keys, comments and optimizer statistics, and supports sample data previews. It uses the pure-Go `github.com/sijms/go-ora/v2` driver, so it needs no Oracle Instant Client.

## Naming

Oracle users treat a schema as the database, so each schema becomes a `Database` asset named after the schema (`HR`). Tables, views, materialized views and procedures are named `SCHEMA.OBJECT` (`HR.EMPLOYEES`), the same shape the OpenMetadata and Trino plugins use for Oracle, so an Oracle table catalogued by any of the three lands on one asset. Identifiers keep the case Oracle stores them in, which is upper case unless they were created quoted.

Sequences are not discovered.

## Lineage

- `CONTAINS` from each schema's `Database` asset to its tables, views and procedures.
- `FOREIGN_KEY` from the referencing table to the referenced table, including references into another discovered schema.
- `VIEW_OF` from each base table to the view or materialized view that reads it, extracted from the view's SQL.

## Required Permissions

`ALL_*` dictionary views only list objects the user can access. Procedures, functions and packages need `EXECUTE ANY PROCEDURE` (or an `EXECUTE` grant per object) to show up. Row counts and sizes come from optimizer statistics, so run `DBMS_STATS.GATHER_SCHEMA_STATS` for schemas that have never been analyzed.

```sql
CREATE USER marmot_reader IDENTIFIED BY "your-password";
GRANT CREATE SESSION TO marmot_reader;
GRANT SELECT ANY TABLE TO marmot_reader;
```

With `use_dba_views: true` the plugin reads `DBA_*` views instead, which list every object regardless of grants and need `SELECT ANY DICTIONARY`:

```sql
GRANT SELECT ANY DICTIONARY TO marmot_reader;
```

## Example Configuration

```yaml

host: "oracle-prod.internal"
port: 1521
user: "marmot_reader"
password: "secure_password"
service_name: "ORCLPDB1"
schemas:
  - "HR"
  - "SALES"
include_columns: true
include_views: true
include_materialized_views: true
include_procedures: true
discover_foreign_keys: true
include_statistics: true
tags:
  - "oracle"
  - "production"

```

Connect by SID instead of service name with `sid: "ORCL"` (set one of the two, not both). For TLS listeners set `ssl: true` and, when the server certificate is not in the system trust store, point `wallet_path` at an Oracle wallet directory.

## Configuration
The following configuration options are available:

| Property | Type | Required | Description |
|----------|------|----------|-------------|
| discover_foreign_keys | bool | false | Whether to discover foreign key relationships |
| exclude_schemas | []string | false | Schemas to skip (defaults to the schemas Oracle ships with, such as SYS and SYSTEM) |
| external_links | []ExternalLink | false | External links to show on all assets |
| filter | Filter | false | Filter discovered assets by name (regex) |
| host | string | true | Oracle listener hostname or IP address |
| include_columns | bool | false | Whether to include column information in table metadata |
| include_materialized_views | bool | false | Whether to discover materialized views |
| include_procedures | bool | false | Whether to discover procedures, functions and packages |
| include_statistics | bool | false | Whether to include row counts and sizes from optimizer statistics |
| include_views | bool | false | Whether to discover views |
| password | string | true | Password for authentication |
| port | int | false | Oracle listener port |
| schemas | []string | false | Schemas to discover (every schema not maintained by Oracle when empty) |
| service_name | string | false | Service name to connect to, for example FREEPDB1 (required unless sid is set) |
| sid | string | false | System identifier to connect to instead of a service name |
| ssl | bool | false | Connect over TCPS (TLS) |
| ssl_verify | bool | false | Verify the server certificate when ssl is enabled |
| tags | TagsConfig | false | Tags to apply to discovered assets |
| use_dba_views | bool | false | Read DBA_* dictionary views instead of ALL_* (needs SELECT ANY DICTIONARY) |
| user | string | true | Username for authentication |
| wallet_path | string | false | Path to an Oracle wallet directory (TCPS certificates or wallet authentication) |

## Available Metadata

The following metadata fields are available:

| Field | Type | Description |
|-------|------|-------------|
| build_mode | string | Materialized view build mode (IMMEDIATE, DEFERRED) |
| column_name | string | Column name |
| comment | string | Table or view comment |
| compression | string | Table compression setting |
| container | string | Container or pluggable database name (USERENV CON_NAME) |
| created | string | When the schema user or object was created |
| data_type | string | Data type as DESCRIBE shows it, for example NUMBER(10,2) or VARCHAR2(50 CHAR) |
| db_name | string | Database name (USERENV DB_NAME) |
| default_expression | string | Default value expression |
| description | string | Column comment |
| host | string | Oracle listener hostname |
| iot | bool | Whether the table is index-organized |
| is_identity | bool | Whether the column is an identity column |
| is_nullable | bool | Whether null values are allowed |
| is_primary_key | bool | Whether the column is part of the primary key |
| is_virtual | bool | Whether the column is computed from an expression |
| last_analyzed | string | When optimizer statistics were last gathered |
| last_ddl_time | string | When the procedure, function or package was last changed |
| last_refresh | string | When the materialized view was last refreshed |
| materialized | bool | Whether the view is materialized |
| materialized_view_count | int | Number of materialized views in the schema |
| num_rows | int64 | Row count from optimizer statistics |
| object_name | string | Procedure, function or package name as stored by Oracle |
| object_type | string | Object type (table, view, materialized_view, procedure, function, package) |
| oracle_version | string | Oracle version banner |
| partitioned | bool | Whether the table is partitioned |
| port | int | Oracle listener port |
| refresh_method | string | Materialized view refresh method (COMPLETE, FAST, FORCE) |
| refresh_mode | string | Materialized view refresh mode (DEMAND, COMMIT) |
| schema | string | Schema that owns the object |
| service_name | string | Service name the plugin connected to |
| sid | string | SID the plugin connected to, when configured |
| staleness | string | Materialized view staleness (FRESH, STALE, NEEDS_COMPILE) |
| status | string | Compilation status (VALID, INVALID) |
| table_count | int | Number of tables in the schema |
| table_name | string | Object name as stored by Oracle |
| tablespace | string | Tablespace holding the table |
| temporary | bool | Whether the table is a global temporary table |
| text_length | int64 | Length of the view definition |
| view_count | int | Number of views in the schema |
