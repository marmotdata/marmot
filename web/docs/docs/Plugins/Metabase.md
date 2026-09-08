---
title: Metabase
description: Discovers dashboards, charts and models from Metabase, with lineage from the tables they read.
status: experimental
---

# Metabase

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


The Metabase plugin discovers dashboards, questions, metrics and models from a Metabase instance through its REST API, and links them to the warehouse tables they read.

Dashboards become Dashboard assets, questions and metrics become Chart assets, and models become Data Model Object assets. Each is named by the path of the collection holding it and its own name, for example `Marketing/Weekly/Signups by channel`; items in the root collection keep their bare name. Native SQL cards carry their query, and every asset links back to Metabase.

## Lineage

Every dashboard is linked to the cards placed on it. Each card is linked to the tables it reads: query builder cards name their source table and joins, and native SQL cards are scanned for the tables after `FROM` and `JOIN`, matched against the tables Metabase has synced for the card's database. Those tables also feed every dashboard showing the card, and a card built on a model or another question is linked to it.

Tables are addressed as the warehouse's own Marmot plugin addresses them, for example `mrn://table/postgresql/orders` for a Postgres table, so the edges land on the assets that plugin creates. This plugin creates no table assets; run the warehouse plugin as well to see the full graph.

## Authentication

Create an API key in Metabase under Settings, Authentication, API keys (Metabase 0.49 and newer) and set it as `api_key`. Without an API key the plugin logs in with `username` and `password`.



## Example Configuration

```yaml

host: "https://metabase.example.com"
api_key: "${METABASE_API_KEY}"
include_charts: true
include_models: true
discover_lineage: true
include_archived: false
filter:
  include:
    - "^Marketing/.*"
  exclude:
    - ".*/Scratch/.*"
tags:
  - "metabase"
  - "bi"

```

## Configuration
The following configuration options are available:

| Property | Type | Required | Description |
|----------|------|----------|-------------|
| api_key | string | false | API key (Metabase 0.49 and newer) |
| discover_lineage | bool | false | Link cards to the tables and models they read |
| external_links | []ExternalLink | false | External links to show on all assets |
| filter | Filter | false | Filter discovered assets by name (regex) |
| host | string | true | Metabase URL, for example https://metabase.example.com |
| include_archived | bool | false | Include archived dashboards and cards |
| include_charts | bool | false | Discover questions and metrics as Chart assets |
| include_models | bool | false | Discover models as Data Model Object assets |
| password | string | false | Password for session login |
| tags | TagsConfig | false | Tags to apply to discovered assets |
| username | string | false | Username for session login, used when no API key is set |
| verify_ssl | bool | false | Verify the server's TLS certificate |

## Available Metadata

The following metadata fields are available:

| Field | Type | Description |
|-------|------|-------------|
| archived | bool | Whether the dashboard or card is archived |
| card_count | int | Number of cards placed on the dashboard |
| card_type | string | Card kind: question, metric or model |
| chart_type | string | Normalised chart type (Table, Bar, Line, Pie, Area, Scatter, Map, Gauge, Text, Other) |
| collection | string | Path of the collection holding the dashboard or card |
| collection_id | int | Id of the collection holding the dashboard or card |
| created_at | string | Creation timestamp |
| creator_id | int | Id of the user who created the dashboard or card |
| database | string | Name of the Metabase database the card queries |
| database_id | int | Id of the Metabase database the card queries |
| display | string | Metabase visualisation, for example bar or scalar |
| id | int | Dashboard or card id in Metabase |
| query_type | string | Whether the card runs SQL (native) or the query builder (query) |
| table_id | int | Id of the card's source table, for query builder cards |
| updated_at | string | Last update timestamp |
| url | string | Link to the dashboard or card in Metabase |
