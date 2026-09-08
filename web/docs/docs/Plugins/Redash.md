---
title: Redash
description: Discovers dashboards, charts, queries and data sources from Redash.
status: experimental
---

# Redash

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


The Redash plugin discovers dashboards, charts, queries and data sources from a Redash instance. It reads the REST API with a user API key, which is shown on the Redash user profile page.

## Assets

A dashboard becomes a Dashboard, each visualization widget on it becomes a Chart named `<dashboard name>/<visualization name>`, each saved query becomes a Data Model Object carrying its SQL, and each connection becomes a DataSource. Redash allows two dashboards or two queries to share a name; a repeat gets its Redash id appended, for example `Revenue (2)`.

Redash dashboards have no description field. The content of any text widgets is recorded in the `notes` metadata field instead.

## Lineage

Dashboard CONTAINS Chart, Data Model Object FEEDS Chart, and DataSource FEEDS Data Model Object.

Table references are read out of each query's SQL and linked as Table FEEDS Data Model Object. The table name is built with the provider and naming rule of the plugin that owns the queried system, so a query against a Postgres data source links to the same table asset the PostgreSQL plugin creates. A data source type Marmot has no identity for produces no table edge. This lineage rides on the Data Model Object assets, so it needs `include_queries`.

## Redash versions

The dashboard URL changed in Redash 10, so the plugin reads the version from `/api/session` and picks the matching form. A Redash that does not report its version is treated as current.

Archived dashboards are never returned by the Redash API, so `include_archived` only affects queries.

## Example Configuration

```yaml

host: "https://redash.example.com"
api_key: "${REDASH_API_KEY}"
include_queries: true
include_data_sources: true
include_drafts: true
include_archived: false
discover_lineage: true
page_size: 100
filter:
  include:
    - "^Revenue.*"
  exclude:
    - ".*Scratch$"
tags:
  - "redash"
  - "bi"

```

## Configuration
The following configuration options are available:

| Property | Type | Required | Description |
|----------|------|----------|-------------|
| api_key | string | true | Redash user API key, found on the user profile page |
| discover_lineage | bool | false | Whether to link dashboards, charts, queries and the tables they read |
| external_links | []ExternalLink | false | External links to show on all assets |
| filter | Filter | false | Filter discovered assets by name (regex) |
| host | string | true | Redash base URL, for example https://redash.example.com |
| include_archived | bool | false | Whether to include archived dashboards and queries |
| include_data_sources | bool | false | Whether to catalogue data sources |
| include_drafts | bool | false | Whether to include unpublished dashboards and queries |
| include_queries | bool | false | Whether to catalogue saved queries |
| page_size | int | false | Results to request per API page |
| tags | TagsConfig | false | Tags to apply to discovered assets |
| verify_ssl | bool | false | Whether to verify the server TLS certificate |

## Available Metadata

The following metadata fields are available:

| Field | Type | Description |
|-------|------|-------------|
| account | string | Account identifier, for Snowflake |
| catalog | string | Catalog name, for Trino, Presto and Databricks |
| chart_type | string | Normalised chart shape, read from options.globalSeriesType for a CHART |
| created_at | string | Creation timestamp |
| dashboard | string | Name of the dashboard the chart sits on |
| database | string | Database name, for sources that call it that |
| data_source | string | Name of the data source the query runs against |
| data_source_id | int | Redash data source id |
| dataset | string | Dataset name |
| dbname | string | Database name |
| description | string | Query description |
| host | string | Connection host |
| id | int | Redash object id |
| is_archived | bool | Whether the object is archived |
| is_draft | bool | Whether the object is unpublished |
| is_hidden | bool | Whether the widget is hidden on the dashboard |
| notes | string | Text widget content, joined with blank lines |
| owner | string | Display name of the user who created the object |
| parameters | []object | Query parameters, each with a name, title and type |
| paused | bool | Whether query execution is paused |
| pause_reason | string | Why the data source is paused |
| port | int | Connection port |
| project | string | Google Cloud project, for BigQuery |
| query_count | int | Number of distinct queries the dashboard reads |
| query_id | int | Id of the query the chart renders |
| query_name | string | Name of the query the chart renders |
| redash_version | string | Redash release the dashboard was read from |
| region | string | Cloud region |
| runtime | float64 | Seconds the last run took |
| schedule | object | Refresh schedule: interval in seconds, time, day of week and until date |
| schema | string | Default schema |
| service_name | string | Oracle service name |
| slug | string | URL slug |
| ssl_mode | string | Connection SSL mode |
| syntax | string | Query syntax the data source accepts, for example sql or json |
| tags | []string | Tags set on the object in Redash |
| type | string | Redash data source type, for example pg, mysql or snowflake |
| updated_at | string | Last update timestamp |
| url | string | Link to the object in Redash |
| user | string | Connection user |
| view_only | bool | Whether the data source is read only for the current user |
| visualization_id | int | Redash visualization id |
| visualization_type | string | Redash visualization type, for example CHART, TABLE or COUNTER |
| warehouse | string | Warehouse name, for Snowflake |
| widget_count | int | Number of widgets on the dashboard, text widgets included |
| widget_id | int | Redash widget id |
