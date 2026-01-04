---
title: PostgreSQL
description: This plugin discovers databases and tables from PostgreSQL instances.
status: experimental
---

# PostgreSQL

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
  href="/docs/Populating/UI"
  buttonText="View Guide"
  variant="secondary"
  icon="mdi:cursor-default-click"
/>


The PostgreSQL plugin discovers databases, schemas, and tables from PostgreSQL instances. It captures column information, table metrics, and foreign key relationships for lineage.

## Required Permissions

The user needs read access to the information schema:

```sql
GRANT CONNECT ON DATABASE your_db TO marmot_reader;
GRANT USAGE ON SCHEMA public TO marmot_reader;
GRANT SELECT ON ALL TABLES IN SCHEMA public TO marmot_reader;
```



## Example Configuration

```yaml

host: "prod-postgres.company.com"
port: 5432
user: "marmot_reader"
password: "secure_password_123"
ssl_mode: "require"
tags:
  - "postgres"
  - "production"

```

## Configuration
The following configuration options are available:

| Property | Type | Required | Description |
|----------|------|----------|-------------|
| database_filter | plugin.Filter | false | Filter configuration for databases |
| discover_foreign_keys | bool | false | Whether to discover foreign key relationships |
| enable_metrics | bool | false | Whether to include table metrics |
| exclude_system_schemas | bool | false | Whether to exclude system schemas (pg_*) |
| external_links | []ExternalLink | false | External links to show on all assets |
| host | string | false | PostgreSQL server hostname or IP address |
| include_columns | bool | false | Whether to include column information in table metadata |
| include_databases | bool | false | Whether to discover databases |
| password | string | false | Password for authentication |
| port | int | false | PostgreSQL server port |
| schema_filter | plugin.Filter | false | Filter configuration for schemas |
| ssl_mode | string | false | SSL mode (disable, require, verify-ca, verify-full) |
| table_filter | plugin.Filter | false | Filter configuration for tables |
| tags | TagsConfig | false | Tags to apply to discovered assets |
| user | string | false | Username for authentication |

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