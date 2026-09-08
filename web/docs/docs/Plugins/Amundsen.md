---
title: Amundsen
description: Imports tables, columns, owners, usage, dashboards and lineage from an Amundsen metadata graph.
status: experimental
---

# Amundsen

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


The Amundsen plugin reads an Amundsen metadata graph straight from its Neo4j over Bolt. It imports tables and views with their columns, owners, tags and badges, dashboards with their charts, read counts as statistics, and the lineage Amundsen records between tables and between a dashboard and the tables behind it.

## Assets land on the technology they belong to

Amundsen is a catalog, so everything in it describes something that lives somewhere else: a table under Amundsen's `postgres` database is a Postgres table. Each entry is projected onto the provider and name the technology's own Marmot plugin uses, so an Amundsen import and a later native run merge onto one asset rather than leaving two half populated ones.

A Postgres table becomes `mrn://table/postgresql/orders`, a Hive one `mrn://table/hive/sales.orders`, a Snowflake one `mrn://table/snowflake/analytics.public.orders`. A technology with no entry in the table keeps its Amundsen name as the provider and is named `schema.table`.

No database, cluster or schema assets are created, because those belong to the technology's own plugin.

## Naming and the cluster

Amundsen's hierarchy is `Database -> Cluster -> Schema -> Table`, one level deeper than Marmot's, and its `Database` holds the technology rather than a database. The cluster enters an asset's name only where the technology's own naming has room for it, which is the leading part of a three part name such as a Snowflake or Presto table. Everywhere else the cluster is an environment label like `prod` or `gold`, so it is recorded under `amundsen.cluster` in metadata instead.

## Statistics

Amundsen counts how often something was read rather than how large it is, so this plugin emits `asset.read_count` and `asset.unique_readers` for every table and dashboard that has been read at least once, not the `asset.row_count` family the database plugins use.

## Connecting

The Neo4j Go driver takes encryption from the address scheme. Set `encrypted: true` and the plugin upgrades `bolt://` to `bolt+s://` for you, or to `bolt+ssc://` when `trust_all_certificates` is also set. An address that already names an encrypted scheme is used as it is.

## Example Configuration

```yaml

uri: "bolt://neo4j.company.com:7687"
username: "neo4j"
password: "secret"
amundsen_url: "https://amundsen.company.com"
include_dashboards: true
tags:
  - "amundsen"

```

## Configuration
The following configuration options are available:

| Property | Type | Required | Description |
|----------|------|----------|-------------|
| amundsen_url | string | false | Address of the Amundsen web app, used to link each asset back to its page |
| database | string | false | Neo4j database holding the Amundsen graph |
| encrypted | bool | false | Connect over TLS |
| external_links | []ExternalLink | false | External links to show on all assets |
| filter | Filter | false | Filter discovered assets by name (regex) |
| include_dashboards | bool | false | Import dashboards and their charts |
| include_descriptions | bool | false | Copy Amundsen descriptions onto assets and columns |
| include_tags | bool | false | Copy Amundsen tags onto assets |
| include_usage | bool | false | Import Amundsen read counts as statistics |
| include_users | bool | false | Record table owners from Amundsen |
| page_size | int | false | Records per query. Every query is paged, so a large graph does not have to fit in memory |
| password | string | true | Neo4j password |
| query_timeout_seconds | int | false | Per-query timeout |
| tags | TagsConfig | false | Tags to apply to discovered assets |
| trust_all_certificates | bool | false | Accept any TLS certificate, including self signed ones |
| uri | string | true | Bolt address of Amundsen's Neo4j, for example bolt://neo4j.company.com:7687 |
| username | string | true | Neo4j username |

## Available Metadata

The following metadata fields are available:

| Field | Type | Description |
|-------|------|-------------|
| amundsen.amundsen_url | string | Dashboard page in the Amundsen web app |
| amundsen.badges | []string | Badges applied to the table or dashboard in Amundsen |
| amundsen.chart_id | string | Chart id in the BI tool |
| amundsen.cluster | string | Amundsen cluster, usually an environment label such as prod or gold |
| amundsen.dashboard | string | Dashboard name |
| amundsen.dashboard_key | string | Amundsen key of the dashboard holding the chart |
| amundsen.database | string | Amundsen database, which holds the technology name rather than a database |
| amundsen.group | string | Dashboard group name |
| amundsen.group_description | string | Dashboard group description |
| amundsen.group_url | string | Dashboard group address in the BI tool |
| amundsen.key | string | Amundsen node key, for example postgres://prod.public/orders |
| amundsen.last_successful_run | string | When the dashboard last refreshed successfully |
| amundsen.last_updated_at | string | When the table last changed, as Amundsen recorded it |
| amundsen.product | string | BI tool the dashboard or chart belongs to |
| amundsen.programmatic_descriptions | []string | Descriptions written by an automated source rather than by a person |
| amundsen.query_names | []string | Names of the queries feeding the dashboard |
| amundsen.schema | string | Schema holding the table |
| amundsen.table | string | Table name as Amundsen records it |
| amundsen.tags | []string | Amundsen tags of type default |
| amundsen.url | string | Page in the Amundsen web app, or the chart address in the BI tool |
| chart_count | int | Number of charts on the dashboard |
| chart_type | string | Chart type, for example bar or line |
| column_name | string | Column name |
| data_type | string | Column type as the source system reports it |
| description | string | Column description |
| is_nullable | bool | Always true: Amundsen does not record nullability |
| owner_emails | []string | Owner email addresses |
| owner_teams | []string | Teams the owners belong to |
| owners | []string | Owner display names |
| schema_description | string | Description of the schema holding the table |
