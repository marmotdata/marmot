---
title: Hive
description: Discovers databases, tables, views and materialized views from Apache Hive through HiveServer2.
status: experimental
---

# Hive

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


The Hive plugin discovers databases, tables, views and materialized views from Apache Hive through HiveServer2. It captures columns with their Hive types and comments, partition and bucket layout, storage formats, table statistics and view queries, and creates lineage from a database to its objects, from base tables to views and along foreign key constraints.

Every table and view is named `database.table`, so `mrn://table/hive/sales.orders` is the orders table in the sales database. Sample data previews are supported.

## Connecting

The plugin connects to HiveServer2 over Thrift, on the binary transport (port 10000) or the HTTP transport (`transport: http`, usually port 10001 and path `cliservice`). `auth` selects the mechanism HiveServer2 is configured with: `NONE` (the default, PLAIN SASL without a password check), `NOSASL`, `LDAP` (username and password) or `KERBEROS`. Kerberos needs a plugin binary built with `go build -tags kerberos`, which links the GSSAPI library; the published binaries are built without it.

Row counts and sizes come from the metastore statistics, so they are as fresh as the last `ANALYZE TABLE` or stats-collecting write. Hive 4 creates non-ACID tables as external tables, and `object_type` reports what Hive reports.

## Example Configuration

```yaml

host: "hive.internal"
port: 10000
auth: "LDAP"
username: "marmot_reader"
password: "secret"
databases:
  - "sales"
  - "marketing"
include_ddl: false
tags:
  - "hive"
  - "warehouse"

```

## Configuration
The following configuration options are available:

| Property | Type | Required | Description |
|----------|------|----------|-------------|
| auth | string | false | Authentication mechanism (NONE, NOSASL, LDAP, KERBEROS) |
| databases | []string | false | Databases to discover (all when empty) |
| exclude_databases | []string | false | Databases to skip |
| external_links | []ExternalLink | false | External links to show on all assets |
| filter | Filter | false | Filter discovered assets by name (regex) |
| host | string | true | HiveServer2 hostname or IP address |
| http_path | string | false | Endpoint path for the http transport |
| include_columns | bool | false | Include column information |
| include_ddl | bool | false | Store the SHOW CREATE TABLE output in metadata |
| include_partitions | bool | false | Count the partitions of partitioned tables |
| include_statistics | bool | false | Include row counts and sizes from table statistics |
| include_views | bool | false | Include views and materialized views |
| kerberos_service_name | string | false | Service part of the HiveServer2 Kerberos principal |
| password | string | false | Password, required for LDAP |
| port | int | false | HiveServer2 port |
| ssl | bool | false | Connect over TLS |
| ssl_skip_verify | bool | false | Skip TLS certificate verification |
| tags | TagsConfig | false | Tags to apply to discovered assets |
| transport | string | false | Thrift transport (binary or http) |
| username | string | false | Username (hive when empty) |

## Available Metadata

The following metadata fields are available:

| Field | Type | Description |
|-------|------|-------------|
| bucket_columns | []string | Columns the table is bucketed by |
| column_name | string | Column name |
| comment | string | Database comment |
| comment | string | Table comment |
| compressed | bool | Whether the storage is marked compressed |
| created | string | Creation time (RFC 3339) |
| data_type | string | Hive data type as declared, complex types included |
| database | string | Database name |
| database | string | Database the object belongs to |
| ddl | string | SHOW CREATE TABLE output, when include_ddl is set |
| default_expression | string | Default value from a DEFAULT constraint |
| description | string | Column comment |
| hive_version | string | Hive version reported by the server |
| host | string | HiveServer2 hostname |
| input_format | string | Hadoop input format class |
| is_nullable | bool | False when a NOT NULL constraint covers the column |
| is_partition_column | bool | Whether the column is a partition column |
| is_primary_key | bool | Whether the column is part of the primary key constraint |
| last_access | string | Last access time (RFC 3339), when Hive tracks it |
| last_ddl | string | Time of the last DDL change (RFC 3339) |
| location | string | Warehouse location of the database |
| location | string | Storage location |
| num_buckets | int | Number of buckets, for bucketed tables |
| num_files | int64 | Number of files, from table statistics |
| object_type | string | Object type (managed, external, view, materialized_view) |
| output_format | string | Hadoop output format class |
| owner | string | Database owner |
| owner | string | Object owner |
| owner_type | string | Owner type (USER, ROLE, GROUP) |
| parameters | map[string]string | Database properties (DBPROPERTIES) |
| parameters | map[string]string | Table properties not surfaced as their own field |
| partition_columns | []string | Partition columns |
| partition_count | int | Number of partitions |
| partition_values | []string | First 20 partition specs (dt=2026-01-01) |
| port | int | HiveServer2 port |
| serde | string | SerDe class |
| sort_columns | []string | Columns each bucket is sorted by |
| table_count | int | Number of tables discovered in the database |
| table_name | string | Table or view name |
| transactional | bool | Whether the table is ACID (transactional) |
| view_count | int | Number of views and materialized views discovered in the database |
