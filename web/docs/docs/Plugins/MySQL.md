---
title: MySQL
description: This plugin discovers databases and tables from MySQL instances.
status: experimental
---

# MySQL

<div class="flex flex-col gap-3 mb-6 pb-6 border-b border-gray-200">
<div class="flex items-center gap-3">
<span class="inline-flex items-center rounded-full px-4 py-2 text-sm font-medium bg-earthy-yellow-300 text-earthy-yellow-900">Experimental</span>
</div>
<div class="flex items-center gap-2">
<span class="text-sm text-gray-500">Creates:</span>
<div class="flex flex-wrap gap-2"><span class="inline-flex items-center rounded-lg px-4 py-2 text-sm font-medium bg-earthy-green-100 text-earthy-green-800 border border-earthy-green-300">Assets</span><span class="inline-flex items-center rounded-lg px-4 py-2 text-sm font-medium bg-earthy-green-100 text-earthy-green-800 border border-earthy-green-300">Lineage</span></div>
</div>
</div>


The MySQL plugin discovers databases and tables from MySQL instances. It captures column information, row counts, and foreign key relationships for lineage.

## Required Permissions

The user needs read access to the information schema:

```sql
CREATE USER 'marmot_reader'@'%' IDENTIFIED BY 'your-password';
GRANT SELECT ON your_database.* TO 'marmot_reader'@'%';
GRANT SELECT ON information_schema.* TO 'marmot_reader'@'%';
```


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
| database | string | false | Database name to connect to |
| discover_foreign_keys | bool | false | Whether to discover foreign key relationships |
| external_links | []ExternalLink | false | External links to show on all assets |
| host | string | false | MySQL server hostname or IP address |
| include_columns | bool | false | Whether to include column information in table metadata |
| include_row_counts | bool | false | Whether to include approximate row counts |
| password | string | false | Password for authentication |
| port | int | false | MySQL server port |
| table_filter | plugin.Filter | false | Filter configuration for tables |
| tags | TagsConfig | false | Tags to apply to discovered assets |
| tls | string | false | TLS configuration (false, true, skip-verify, preferred) |
| user | string | false | Username for authentication |

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