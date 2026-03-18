---
title: Trino
description: Discovers catalogs, schemas, tables, and views from Trino clusters with optional AI enrichment.
status: experimental
---

# Trino

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


The Trino plugin discovers all catalogs (connected data sources like PostgreSQL, Hive, Iceberg, S3, etc.), their schemas, and tables/views.

## Required Permissions

The connecting user needs `SELECT` access to `system.metadata.catalogs`, `system.metadata.table_comments`, and each catalog's `information_schema`. A read-only user with access to these system tables is sufficient.

## AI Enrichment

When your Trino instance has an [AI connector](https://trino.io/docs/current/connector/ai.html) configured, the plugin can automatically enrich discovered assets:

- **Auto-generate descriptions** (`ai_generate_descriptions: true`) — Uses the AI connector's `ai_gen` function to produce one-sentence descriptions for tables that have no comment.
- **Auto-classify tables** (`ai_classify_tables: true`) — Uses the AI connector's `ai_classify` function to assign a category label (e.g., `analytics`, `pii`, `financial`) to each table, added as a tag like `ai-category:pii`.

### AI Setup

1. Configure an AI connector in your Trino installation (e.g., `ai.properties`)
2. Set `ai_catalog` to the catalog name of that connector
3. Enable `ai_generate_descriptions` and/or `ai_classify_tables`
4. Optionally customise `ai_classify_labels` and `ai_max_enrichments`

AI enrichment is best-effort - failures are logged as warnings but do not prevent normal discovery from completing.re logged as warnings but do not prevent normal discovery from completing.



## Example Configuration

```yaml

host: "trino.company.com"
port: 8080
user: "marmot_reader"
secure: false
exclude_catalogs:
  - "system"
  - "jmx"
tags:
  - "trino"
  - "production"

```

## Configuration
The following configuration options are available:

| Property | Type | Required | Description |
|----------|------|----------|-------------|
| access_token | string | false | JWT bearer token |
| ai_catalog | string | false | Name of the AI connector catalog (empty = disabled) |
| ai_classify_labels | []string | false | Custom classification labels |
| ai_classify_tables | bool | false | Auto-classify tables into categories |
| ai_generate_descriptions | bool | false | Auto-generate descriptions for undocumented tables |
| ai_max_enrichments | int | false | Max tables to enrich with AI (0 = unlimited) |
| catalog | string | false | Specific catalog to discover (all if empty) |
| exclude_catalogs | []string | false | Catalogs to skip |
| external_links | []ExternalLink | false | External links to show on all assets |
| filter | Filter | false | Filter discovered assets by name (regex) |
| host | string | false | Trino coordinator hostname |
| include_catalogs | bool | false | Create catalog-level assets |
| include_columns | bool | false | Include column info in table metadata |
| include_stats | bool | false | Collect table statistics (can be slow) |
| password | string | false | Password (requires HTTPS) |
| port | int | false | Trino coordinator port |
| secure | bool | false | Use HTTPS |
| ssl_cert_path | string | false | Path to TLS certificate file |
| tags | TagsConfig | false | Tags to apply to discovered assets |
| user | string | false | Username for authentication |

## Available Metadata

The following metadata fields are available:

| Field | Type | Description |
|-------|------|-------------|
| catalog | string | Parent catalog name |
| catalog | string | Parent catalog name |
| catalog_name | string | Trino catalog name |
| column_name | string | Column name |
| comment | string | Table comment |
| data_type | string | Column data type |
| is_nullable | string | YES or NO |
| ordinal_position | int | Column position |
| row_count | int64 | Estimated row count |
| schema | string | Parent schema name |
| schema_name | string | Schema name |
| table_name | string | Table or view name |
| table_type | string | BASE TABLE or VIEW |