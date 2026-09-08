---
title: CockroachDB
description: Discovers databases, tables, views and foreign key relationships from CockroachDB clusters.
status: experimental
---

# CockroachDB

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


The CockroachDB plugin discovers databases, tables, views and materialized views from a CockroachDB cluster. It captures columns (including computed column expressions and comments), row and column counts, table sizes, partitioning, foreign key relationships and, for each view, a lineage edge from every table it reads to the view.

One cluster holds many databases, so tables and views are named `database.schema.table`. Every database except `system` is discovered by default; set `database` to discover one, or `exclude_databases` to skip more.

Hidden columns CockroachDB adds on its own (the `rowid` of a table without a primary key and the shard column of a hash-sharded index) are left out.

## Connection

The plugin speaks the PostgreSQL wire protocol over the SQL port (26257 by default). Set `ssl_mode` to `verify-full` with `ssl_root_cert` for secure clusters, and `ssl_cert` plus `ssl_key` for certificate authentication. Leave `password` empty on insecure clusters.



## Example Configuration

```yaml

host: "crdb-prod.internal"
port: 26257
user: "marmot_reader"
password: "secure_password_123"
ssl_mode: "verify-full"
ssl_root_cert: "/etc/marmot/certs/ca.crt"
exclude_databases:
  - "system"
  - "defaultdb"
tags:
  - "cockroachdb"
  - "production"

```

## Configuration
The following configuration options are available:

| Property | Type | Required | Description |
|----------|------|----------|-------------|
| database | string | false | Discover only this database. Leave empty to discover every database |
| discover_foreign_keys | bool | false | Whether to discover foreign key relationships |
| exclude_databases | []string | false | Databases to skip when discovering every database (default: `system`) |
| external_links | []ExternalLink | false | External links to show on all assets |
| filter | Filter | false | Filter discovered assets by name (regex) |
| host | string | true | CockroachDB node hostname or IP address |
| include_columns | bool | false | Whether to include column information in table metadata |
| include_statistics | bool | false | Whether to include row counts, column counts and table sizes |
| include_views | bool | false | Whether to discover views and materialized views |
| password | string | false | Password for the SQL user. Leave empty on insecure clusters or with certificate authentication |
| port | int | false | SQL port (default: 26257) |
| ssl_cert | string | false | Path to the client certificate, for certificate authentication |
| ssl_key | string | false | Path to the client certificate key |
| ssl_mode | string | false | SSL mode (disable, require, verify-ca, verify-full) |
| ssl_root_cert | string | false | Path to the CA certificate that signed the node certificates |
| tags | TagsConfig | false | Tags to apply to discovered assets |
| user | string | true | SQL user |

## Available Metadata

The following metadata fields are available:

| Field | Type | Description |
|-------|------|-------------|
| column_name | string | Column name |
| comment | string | Object comment |
| data_type | string | CockroachDB data type |
| database | string | Database name |
| default_expression | string | Default value expression |
| description | string | Column comment |
| estimated_row_count | int64 | Row count estimated from the optimizer's table statistics |
| generation_expression | string | Expression of a computed column |
| host | string | CockroachDB node hostname |
| is_nullable | bool | Whether null values are allowed |
| is_primary_key | bool | Whether the column is part of the primary key |
| materialized | bool | Whether a view is materialized |
| object_type | string | Object type (table, view, materialized_view, foreign_table) |
| owner | string | Object owner |
| partition_columns | string | Columns the table is partitioned on, comma separated |
| partitioned | bool | Whether the table's primary index is partitioned |
| port | int | SQL port |
| schema | string | Schema name |
| server_version | string | CockroachDB version string |
| table_name | string | Table or view name |
