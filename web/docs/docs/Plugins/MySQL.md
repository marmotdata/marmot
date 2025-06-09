---
title: MySQL
description: This plugin discovers databases and tables from MySQL instances.
status: experimental
---

# MySQL

This plugin discovers databases and tables from MySQL instances.

**Status:** experimental

## Example Configuration

```yaml

host: "mysql-prod.internal"
port: 3306
user: "marmot_user"
password: "mysql_secure_pass"
database: "ecommerce"
tls: "true"
tags:
  - "mysql"
  - "ecommerce"

```

## Configuration
The following configuration options are available:

| Property | Type | Required | Description |
|----------|------|----------|-------------|
| host | string | false | MySQL server hostname or IP address |
| port | int | false | MySQL server port (default: 3306) |
| user | string | false | Username for authentication |
| password | string | false | Password for authentication |
| database | string | false | Database name to connect to |
| tls | string | false | TLS configuration (false, true, skip-verify, preferred) |
| include_columns | bool | false | Whether to include column information in table metadata |
| include_row_counts | bool | false | Whether to include approximate row counts |
| discover_foreign_keys | bool | false | Whether to discover foreign key relationships |
| table_filter | plugin.Filter | false | Filter configuration for tables |

## Available Metadata

The following metadata fields are available:

| Field | Type | Description |
|-------|------|-------------|
| character_set | string | Character set |
| charset | string | Character set |
| collation | string | Table collation |
| collation | string | Collation |
| column_default | string | Default value |
| column_name | string | Column name |
| column_type | string | Full column type definition |
| comment | string | Object comment/description |
| comment | string | Column comment/description |
| constraint_name | string | Foreign key constraint name |
| created | string | Creation timestamp |
| data_length | int64 | Data size in bytes |
| data_type | string | Data type |
| database | string | Database name |
| delete_rule | string | Delete rule (CASCADE, RESTRICT, etc.) |
| engine | string | Storage engine |
| host | string | MySQL server hostname |
| index_length | int64 | Index size in bytes |
| is_auto_increment | bool | Whether column auto-increments |
| is_nullable | bool | Whether null values are allowed |
| is_primary_key | bool | Whether column is part of primary key |
| object_type | string | Object type (table, view) |
| port | int | MySQL server port |
| row_count | int64 | Approximate row count |
| schema | string | Schema name |
| source_column | string | Column in the referencing table |
| source_schema | string | Schema of the referencing table |
| source_table | string | Name of the referencing table |
| table_name | string | Object name |
| target_column | string | Column in the referenced table |
| target_schema | string | Schema of the referenced table |
| target_table | string | Name of the referenced table |
| update_rule | string | Update rule (CASCADE, RESTRICT, etc.) |
| updated | string | Last update timestamp |
| version | string | MySQL version |