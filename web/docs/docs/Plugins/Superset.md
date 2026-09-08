---
title: Superset
description: Discovers dashboards, charts, datasets and database connections from Apache Superset.
status: experimental
---

# Superset

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


The Superset plugin discovers dashboards, charts, datasets and database connections from Apache Superset through its REST API. It logs in with a username and password and reads everything the user can see.

Dashboards are linked to the charts on them, datasets to the charts that read them, and each database connection to its datasets. A dataset is also linked to the table it reads in the connected database, using the identity that database's own Marmot plugin gives the table, so a Superset dataset on `public.orders` in a PostgreSQL connection lands next to the PostgreSQL plugin's `orders` table. Virtual datasets are linked to the tables named after `FROM` and `JOIN` in their SQL.

## Table lineage

Table edges are emitted for these backends: PostgreSQL, MySQL, MariaDB, BigQuery, ClickHouse, SQLite, DuckDB, Athena (catalogued as Glue), Snowflake, Redshift, SQL Server, CockroachDB, Oracle, Druid, Trino and Databricks. Other backends are catalogued without a table edge. The edge shows up in Marmot once the database has been ingested with its own plugin.



## Example Configuration

```yaml

host: "https://superset.example.com"
username: "marmot"
password: "superset_secure_pass"
provider: "db"
include_draft: false
tags:
  - "superset"
  - "bi"

```

## Configuration
The following configuration options are available:

| Property | Type | Required | Description |
|----------|------|----------|-------------|
| discover_lineage | bool | false | Link dashboards, charts, datasets and the tables they read |
| external_links | []ExternalLink | false | External links to show on all assets |
| filter | Filter | false | Filter discovered assets by name (regex) |
| host | string | true | Superset URL (e.g. https://superset.example.com) |
| include_charts | bool | false | Discover charts |
| include_databases | bool | false | Discover database connections |
| include_datasets | bool | false | Discover datasets |
| include_draft | bool | false | Include unpublished (draft) dashboards |
| page_size | int | false | Objects per API page |
| password | string | true | Password to log in with |
| provider | string | false | Authentication provider (db or ldap) |
| tags | TagsConfig | false | Tags to apply to discovered assets |
| username | string | true | Username to log in with |
| verify_ssl | bool | false | Verify the server's TLS certificate |

## Available Metadata

The following metadata fields are available:

| Field | Type | Description |
|-------|------|-------------|
| allow_dml | bool | Whether SQL Lab may run DML against the database |
| backend | string | SQLAlchemy dialect of the database (postgresql, snowflake, ...) |
| changed_on | string | Last change time (UTC) |
| chart_count | int | Number of charts on the dashboard |
| chart_type | string | Normalised chart type (Table, Line, Bar, Pie, ...) |
| column_name | string | Column name |
| dashboards | []string | Titles of the dashboards the chart is on |
| data_type | string | Column data type |
| database | string | Database connection name, or the database a connection opens |
| database_id | int | Id of the database connection |
| dataset | string | Dataset the chart reads (schema.table) |
| datasource_id | int | Id of the dataset the chart reads |
| datasource_type | string | Kind of datasource (table) |
| description | string | Dataset or column description |
| driver | string | SQLAlchemy driver |
| expose_in_sqllab | bool | Whether the database is available in SQL Lab |
| expression | string | SQL expression of a calculated column |
| host | string | Database host |
| id | int | Object id in Superset |
| kind | string | physical (a table) or virtual (a SQL query) |
| owners | []string | Names of the owners |
| port | string | Database port |
| published | bool | Whether the dashboard is published |
| schema | string | Schema the dataset lives in |
| slug | string | Dashboard URL slug |
| sqlalchemy_uri | string | Connection URI with the password masked |
| status | string | Dashboard status (published, draft) |
| table_name | string | Table name, or the virtual dataset's name |
| tags | []string | Tags applied in Superset |
| url | string | Link to the object in Superset |
| viz_type | string | Superset visualisation type |
