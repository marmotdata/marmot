---
title: Grafana
description: Discovers dashboards, panels and data sources from Grafana, with lineage to the tables SQL panels read.
status: experimental
---

# Grafana

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


The Grafana plugin discovers dashboards, their panels and the configured data sources from a Grafana instance. Every dashboard becomes a Dashboard asset named by its folder and title (`Sales/Orders overview`), every panel a Chart named by its dashboard and panel title, and every data source a DataSource asset. Rows are flattened, text panels are skipped and library panels are resolved to the shared panel they stand for.

A chart records the query it runs when all of its targets speak one language: SQL for SQL data sources, PromQL for Prometheus and LogQL for Loki.

## Lineage

Dashboards contain their charts, and each data source feeds the charts that query it. Panels backed by a PostgreSQL, MySQL, SQL Server or ClickHouse data source are also linked to the tables their SQL reads, named the way the native Marmot plugin for that database names them, so the edges land on the assets those plugins discover. Template variables and Grafana macros in a query never produce an edge.

## Authentication

Create a service account in Grafana (Administration, Service accounts) with the Viewer role and add a token to it. Tokens start with `glsa_`. Data source credentials are never read.



## Example Configuration

```yaml

host: "https://grafana.company.com"
api_key: "glsa_..."
include_panels: true
include_datasources: true
discover_lineage: true
page_size: 100
verify_ssl: true
tags:
  - "grafana"

```

## Configuration
The following configuration options are available:

| Property | Type | Required | Description |
|----------|------|----------|-------------|
| api_key | string | true | Token of a Grafana service account |
| discover_lineage | bool | false | Link charts and dashboards to the tables their SQL queries read |
| external_links | []ExternalLink | false | External links to show on all assets |
| filter | Filter | false | Filter discovered assets by name (regex) |
| host | string | true | Grafana URL, for example https://grafana.company.com |
| include_datasources | bool | false | Discover data sources as DataSource assets |
| include_panels | bool | false | Discover dashboard panels as Chart assets |
| page_size | int | false | Dashboards per search request |
| tags | TagsConfig | false | Tags to apply to discovered assets |
| verify_ssl | bool | false | Verify the TLS certificate of the Grafana host |

## Available Metadata

The following metadata fields are available:

| Field | Type | Description |
|-------|------|-------------|
| access | string | Access mode (proxy or direct) |
| chart_type | string | Normalised chart type (Line, Table, Bar, ...) |
| created | string | When the dashboard was created |
| created_by | string | User who created the dashboard |
| dashboard | string | Title of the dashboard holding the panel |
| dashboard_uid | string | Uid of the dashboard holding the panel |
| database | string | Database the data source connects to |
| datasource | string | Name of the data source the panel queries |
| datasource_type | string | Type of the data source the panel queries |
| datasource_uid | string | Uid of the data source the panel queries |
| folder | string | Title of the folder holding the dashboard |
| folder_uid | string | Uid of the folder holding the dashboard |
| id | int64 | Dashboard or data source numeric id |
| is_default | bool | Whether this is the default data source |
| panel_count | int | Number of panels, not counting rows and text panels |
| panel_id | int64 | Panel id within the dashboard |
| panel_type | string | Grafana panel type, for example timeseries |
| provisioned | bool | Whether the dashboard is managed by provisioning |
| read_only | bool | Whether the data source is read-only in Grafana |
| refresh | string | Auto refresh interval |
| schema_version | int | Dashboard JSON schema version |
| tags | []string | Tags set on the dashboard in Grafana |
| target_count | int | Number of queries the panel runs |
| time_from | string | Start of the default time range |
| time_to | string | End of the default time range |
| type | string | Data source plugin type, for example grafana-postgresql-datasource |
| type_name | string | Data source plugin display name |
| uid | string | Dashboard or data source uid |
| updated | string | When the dashboard was last saved |
| updated_by | string | User who last saved the dashboard |
| url | string | URL of the dashboard, panel or data source address |
| version | int | Dashboard version number |
