---
title: PostgreSQL
description: This plugin discovers databases and tables from PostgreSQL instances.
status: experimental
---

# PostgreSQL

This plugin discovers databases and tables from PostgreSQL instances.

**Status:** experimental

## Example Configuration

```yaml

host: "localhost"
port: 5432
user: "postgres"
password: "mysecretpassword"
ssl_mode: "disable"
include_databases: true
include_columns: true
include_row_counts: true
discover_foreign_keys: true
exclude_system_schemas: true
schema_filter:
  include:
    - "^public$"
    - "^app_.*"
  exclude:
    - "^test_.*"
table_filter:
  include:
    - ".*"
  exclude:
    - "^temp_.*"
database_filter:
  include:
    - ".*"
  exclude:
    - "^template.*"
tags:
  - "postgres"
  - "database"

```

## Configuration
The following configuration options are available:

| Property | Type | Required | Description |
|----------|------|----------|-------------|
| host | string | false | PostgreSQL server hostname or IP address |
| port | int | false | PostgreSQL server port (default: 5432) |
| user | string | false | Username for authentication |
| password | string | false | Password for authentication |
| ssl_mode | string | false | SSL mode (disable, require, verify-ca, verify-full) |
| include_databases | bool | false | Whether to discover databases |
| include_columns | bool | false | Whether to include column information in table metadata |
| include_row_counts | bool | false | Whether to include approximate row counts (requires analyze) |
| discover_foreign_keys | bool | false | Whether to discover foreign key relationships |
| schema_filter | plugin.Filter | false | Filter configuration for schemas |
| table_filter | plugin.Filter | false | Filter configuration for tables |
| database_filter | plugin.Filter | false | Filter configuration for databases |
| exclude_system_schemas | bool | false | Whether to exclude system schemas (pg_*) |

## Available Metadata

The following metadata fields are available:

| Field | Type | Description |
|-------|------|-------------|
| allow_connections | bool | Whether connections to this database are allowed |
| collate | string | Database collation |
| column_default | string | Default value expression |
| column_name | string | Column name |
| comment | string | Column comment/description |
| comment | string | Object comment/description |
| connection_limit | int | Maximum allowed connections |
| constraint_name | string | Foreign key constraint name |
| created | string | Creation timestamp |
| ctype | string | Database character classification |
| data_type | string | Data type |
| database | string | Database name |
| encoding | string | Database encoding |
| host | string | PostgreSQL server hostname |
| is_nullable | bool | Whether null values are allowed |
| is_primary_key | bool | Whether column is part of primary key |
| is_template | bool | Whether database is a template |
| object_type | string | Object type (table, view, materialized_view) |
| owner | string | Object owner |
| port | int | PostgreSQL server port |
| row_count | int64 | Approximate row count |
| schema | string | Schema name |
| size | int64 | Object size in bytes |
| source_column | string | Column in the referencing table |
| source_schema | string | Schema of the referencing table |
| source_table | string | Name of the referencing table |
| table_name | string | Object name |
| target_column | string | Column in the referenced table |
| target_schema | string | Schema of the referenced table |
| target_table | string | Name of the referenced table |