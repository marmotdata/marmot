The Redash plugin discovers dashboards, charts, queries and data sources from a Redash instance.

It reads the REST API with a user API key, sent as `Authorization: Key <api_key>`. The key is found on the Redash user profile page.

## Assets

| Redash object | Marmot type | Name |
|---------------|-------------|------|
| Dashboard | Dashboard | dashboard name |
| Visualization widget | Chart | `<dashboard name>/<visualization name>` |
| Saved query | Data Model Object | query name |
| Data source | DataSource | data source name |

Redash allows two dashboards or two queries to share a name. A repeat gets its Redash id appended, for example `Revenue (2)`.

## Lineage

Dashboard CONTAINS Chart, Data Model Object FEEDS Chart, and DataSource FEEDS Data Model Object.

Table references are read out of each query's SQL and linked as Table FEEDS Data Model Object. The table MRN is built with the provider and naming rule of the plugin that owns the queried system, so the edge lands on that plugin's asset rather than creating a second one. A data source type with no known Marmot identity produces no table edge.

Table lineage rides on the Data Model Object assets, so it needs `include_queries`.

## Redash versions

The dashboard URL changed in Redash 10, so the plugin reads the version from `/api/session` and picks the matching form. A Redash that does not report its version is treated as current.

Archived dashboards are never returned by the Redash API, so `include_archived` only affects queries.
