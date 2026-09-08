---
title: SQL Server
description: Discovers databases, tables, views and routines from Microsoft SQL Server instances.
status: experimental
---

# SQL Server

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


The SQL Server plugin discovers databases, tables, views, stored procedures and functions from Microsoft SQL Server, Azure SQL Database and Azure SQL Edge instances. It uses the pure-Go `github.com/microsoft/go-mssqldb` driver, so no ODBC driver is needed on the host. SQL authentication works with a plain login, and Windows authentication works by giving the login as `DOMAIN\user`.

## Databases and Naming

A SQL Server session is bound to a single database, so the plugin opens one connection per database. By default it discovers every database the login can open, minus `master`, `model`, `msdb` and `tempdb`. Set `database` to discover just one.

One instance holds many databases and two of them can hold the same schema and object name, so tables, views and routines are named `database.schema.object`. Databases are named by themselves.

## Lineage

A database gets a `CONTAINS` edge to every table, view and routine it holds. Foreign keys become `FOREIGN_KEY` edges from the referencing table to the referenced one. View definitions are scanned for the objects they read, which become `VIEW_OF` edges from each base object to the view.

Query history is not read. Usage-based lineage would need the plan cache or Query Store, which are per-database, expensive to scan and often disabled.

## Encryption

`encrypt: true` requires an encrypted connection. A default SQL Server install presents a self-signed certificate, which fails verification, so pair it with `trust_server_certificate: true` or install a certificate the client trusts. `encrypt: false` turns encryption off entirely.

## Comments

SQL Server has no `COMMENT ON`, so descriptions come from `MS_Description` extended properties on schemas, tables, views and columns. Objects without one have no description.

## Example Configuration

```yaml

host: "sqlserver.company.com"
port: 1433
user: "marmot_reader"
password: "secure_password_123"
encrypt: true
trust_server_certificate: false
exclude_databases:
  - "master"
  - "model"
  - "msdb"
  - "tempdb"
tags:
  - "sqlserver"
  - "production"

```

## Configuration
The following configuration options are available:

| Property | Type | Required | Description |
|----------|------|----------|-------------|
| application_intent | string | false | Connect to a read-only replica with ReadOnly |
| connect_timeout_seconds | int | false | Seconds to wait for a connection |
| database | string | false | Discover only this database. Leave empty to discover every database the login can open |
| discover_foreign_keys | bool | false | Whether to discover foreign key relationships |
| encrypt | bool | false | Require an encrypted connection |
| exclude_databases | []string | false | Databases to skip |
| exclude_schemas | []string | false | Schemas to skip |
| external_links | []ExternalLink | false | External links to show on all assets |
| filter | Filter | false | Filter discovered assets by name (regex) |
| host | string | true | SQL Server hostname or IP address |
| include_columns | bool | false | Whether to include column information |
| include_procedures | bool | false | Whether to discover stored procedures and functions |
| include_statistics | bool | false | Whether to collect row counts and table sizes |
| include_views | bool | false | Whether to discover views |
| password | string | true | Password for the login |
| port | int | false | SQL Server port |
| tags | TagsConfig | false | Tags to apply to discovered assets |
| trust_server_certificate | bool | false | Accept the server certificate without verifying it. Needed for self-signed certificates |
| user | string | true | Login to authenticate with. Use DOMAIN\user for Windows authentication |

## Available Metadata

The following metadata fields are available:

| Field | Type | Description |
|-------|------|-------------|
| collation | string | Database collation, or column collation for text types |
| column_name | string | Column name |
| comment | string | MS_Description extended property on the object |
| computed_definition | string | Expression a computed column is derived from |
| created | string | When the object was created |
| data_type | string | Column type as SQL Server declares it, for example nvarchar(100) or decimal(10,2) |
| database | string | Database holding the object |
| database_id | int | Database id within the instance |
| default_expression | string | Default constraint expression |
| description | string | MS_Description extended property on the column |
| edition | string | Instance edition |
| encrypted | bool | Whether the routine body is encrypted and so unreadable |
| host | string | SQL Server hostname or IP address |
| identity_increment | int64 | Step between IDENTITY values |
| identity_seed | int64 | First value an IDENTITY column produces |
| is_computed | bool | Whether the column is computed from other columns |
| is_identity | bool | Whether the column is an IDENTITY column |
| is_nullable | bool | Whether null values are allowed |
| is_persisted | bool | Whether a computed column is stored rather than evaluated on read |
| is_primary_key | bool | Whether the column is part of the primary key |
| modified | string | When the object was last altered |
| object_type | string | Object type (user_table, view) or routine type (stored_procedure, scalar_function, inline_table_function, table_function) |
| owner | string | Login that owns the database |
| port | int | SQL Server port |
| recovery_model | string | Recovery model (SIMPLE, FULL, BULK_LOGGED) |
| schema | string | Schema holding the object |
| schema_comment | string | MS_Description extended property on the schema |
| schema_count | int | Number of discovered schemas |
| server_version | string | Instance product version |
| state | string | Database state, always ONLINE for a discovered database |
| table_count | int | Number of discovered tables |
| table_name | string | Table or view name, without the database and schema |
| view_count | int | Number of discovered views |
