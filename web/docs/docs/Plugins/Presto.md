---
title: Presto
description: Discovers catalogs, schemas, tables and views from Presto clusters.
status: experimental
---

# Presto

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


The Presto plugin discovers catalogs, schemas, tables and views from Presto clusters. Each catalog becomes a Catalog asset that contains its tables and views; view definitions are stored as the view's query and turned into VIEW_OF lineage back to the base tables they read.

Tables of connectors that another Marmot plugin covers (PostgreSQL, MySQL, Hive, Iceberg and the rest of the Trino connector map) are catalogued under that plugin's provider and name shape, so the same table reached through Presto and directly lands on one asset. Everything else, including memory and tpch tables, belongs to Presto under its full `catalog.schema.table` path.

## Required Permissions

The connecting user needs `SELECT` access to `system.metadata.catalogs`, `system.runtime.nodes` and each catalog's `information_schema`. `include_stats` additionally runs `SHOW STATS` per table. A password is only sent over HTTPS, so `secure: true` is required with one.



## Example Configuration

```yaml

host: "presto.company.com"
port: 8080
user: "marmot_reader"
secure: false
exclude_catalogs:
  - "system"
  - "jmx"
exclude_schemas:
  - "scratch"
include_stats: false
tags:
  - "presto"
  - "production"

```

## Configuration
The following configuration options are available:

| Property | Type | Required | Description |
|----------|------|----------|-------------|
| catalog | string | false | Specific catalog to discover (all if empty) |
| exclude_catalogs | []string | false | Catalogs to skip |
| exclude_schemas | []string | false | Schemas to skip in every catalog |
| external_links | []ExternalLink | false | External links to show on all assets |
| filter | Filter | false | Filter discovered assets by name (regex) |
| host | string | true | Presto coordinator hostname |
| include_catalogs | bool | false | Create catalog-level assets |
| include_columns | bool | false | Include column info in table metadata |
| include_stats | bool | false | Collect row counts with SHOW STATS (can be slow) |
| include_views | bool | false | Discover views and their definitions |
| password | string | false | Password (requires HTTPS) |
| port | int | false | Presto coordinator port |
| secure | bool | false | Use HTTPS |
| ssl_cert_path | string | false | Path to TLS certificate file |
| tags | TagsConfig | false | Tags to apply to discovered assets |
| user | string | true | Username for authentication |

## Available Metadata

The following metadata fields are available:

| Field | Type | Description |
|-------|------|-------------|
| catalog | string | Parent catalog name |
| catalog_name | string | Presto catalog name |
| column_name | string | Column name |
| comment | string | Table comment |
| connector_name | string | Connector backing the catalog |
| data_type | string | Presto data type, nested types verbatim |
| default_expression | string | Default value expression |
| description | string | Column comment |
| host | string | Presto coordinator hostname |
| is_nullable | bool | Whether null values are allowed |
| ordinal_position | int | Column position |
| port | int | Presto coordinator port |
| presto_version | string | Presto coordinator version |
| schema | string | Parent schema name |
| schema_count | int | Number of discovered schemas |
| table_count | int | Number of discovered tables |
| table_name | string | Table or view name |
| table_type | string | BASE TABLE or VIEW |
| view_count | int | Number of discovered views |
