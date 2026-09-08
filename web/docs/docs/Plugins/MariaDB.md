---
title: MariaDB
description: Discovers databases, tables, views and sequences from MariaDB servers.
status: experimental
---

# MariaDB

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


The MariaDB plugin discovers one database and its tables, views and sequences from a MariaDB server. It reads column information (including invisible and generated columns), row and size statistics from `information_schema`, foreign key relationships and view lineage. A view is fed by the tables it reads, so `VIEW_OF` edges run from each base table (or view) into the view.

The database is a `Database` asset linked to its tables, views and sequences by `CONTAINS` lineage. Table names are bare (`mrn://table/mariadb/orders`), the same identity the OpenMetadata and Trino plugins use for MariaDB tables, so all three land on one asset.

## Required Permissions

The user needs read access to the database. `SHOW VIEW` is needed to read view definitions:

```sql
CREATE USER 'marmot_reader'@'%' IDENTIFIED BY 'your-password';
GRANT SELECT, SHOW VIEW ON your_database.* TO 'marmot_reader'@'%';
```

Row counts come from `information_schema.TABLES`, which InnoDB refreshes on `ANALYZE TABLE`. They are estimates, not exact counts.



## Example Configuration

```yaml

host: "mariadb-prod.internal"
port: 3306
user: "marmot_reader"
password: "mariadb_secure_pass"
database: "shop"
tls: "true"
include_views: true
include_sequences: true
tags:
  - "mariadb"
  - "shop"

```

## Configuration
The following configuration options are available:

| Property | Type | Required | Description |
|----------|------|----------|-------------|
| database | string | true | Database to discover |
| discover_foreign_keys | bool | false | Whether to discover foreign key relationships |
| external_links | []ExternalLink | false | External links to show on all assets |
| filter | Filter | false | Filter discovered assets by name (regex) |
| host | string | true | MariaDB server hostname or IP address |
| include_columns | bool | false | Whether to include column information in table and view schemas |
| include_row_counts | bool | false | Whether to include approximate row counts in table metadata |
| include_sequences | bool | false | Whether to discover sequences |
| include_statistics | bool | false | Whether to emit row count, size and column count statistics |
| include_views | bool | false | Whether to discover views |
| password | string | false | Password for authentication |
| port | int | false | MariaDB server port |
| tags | TagsConfig | false | Tags to apply to discovered assets |
| tls | string | false | TLS mode (false, true, skip-verify, preferred) |
| user | string | true | Username for authentication |

## Available Metadata

The following metadata fields are available:

| Field | Type | Description |
|-------|------|-------------|
| auto_increment | int64 | Next auto increment value |
| cache_size | int64 | Number of values cached per fetch |
| character_set | string | Default character set of the database, or of a column |
| check_option | string | WITH CHECK OPTION setting (NONE, LOCAL, CASCADED) |
| collation | string | Collation of the database, table or column |
| column_name | string | Column name |
| comment | string | Table comment |
| created | string | Creation timestamp |
| cycle_option | bool | Whether the sequence restarts after reaching its limit |
| data_length | int64 | Data size in bytes |
| data_type | string | Full column type, for example varchar(255) or int(11) |
| database | string | Database name |
| default_expression | string | Default value or expression |
| definer | string | Account that defined the view |
| description | string | Column comment |
| engine | string | Storage engine |
| host | string | MariaDB server hostname |
| increment | int64 | Step between values |
| index_length | int64 | Index size in bytes |
| is_auto_increment | bool | Whether the column auto-increments |
| is_generated | bool | Whether the column is computed from an expression |
| is_invisible | bool | Whether the column is hidden from SELECT * |
| is_nullable | bool | Whether null values are allowed |
| is_primary_key | bool | Whether the column is part of the primary key |
| is_updatable | bool | Whether the view accepts writes |
| maximum_value | int64 | Largest value the sequence produces |
| minimum_value | int64 | Smallest value the sequence produces |
| object_type | string | Object type (table, view, sequence) |
| port | int | MariaDB server port |
| row_count | int64 | Approximate row count |
| schema | string | Schema name (same as the database in MariaDB) |
| security_type | string | SQL SECURITY setting (DEFINER, INVOKER) |
| server_version | string | MariaDB server version |
| start_value | int64 | Value the sequence started at |
| system_versioned | bool | Whether the table keeps row history (WITH SYSTEM VERSIONING) |
| table_name | string | Object name |
| temporary | bool | Whether the table is temporary (only when the server exposes it) |
| updated | string | Last update timestamp |
