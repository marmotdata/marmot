The Grafana plugin discovers dashboards, their panels and the configured data sources from a Grafana instance. Panels backed by a SQL data source are linked to the tables their queries read.

## Assets

| Asset | Name | Notes |
|-------|------|-------|
| Dashboard | `<folder>/<title>`, or `<title>` in the General folder | Description, tags, version, time range and refresh interval from Grafana |
| Chart | `<dashboard name>/<panel title>` | One per panel; untitled panels are `panel-<id>`. Rows are flattened, text panels skipped. SQL, PromQL and LogQL queries are recorded on the chart |
| DataSource | The data source name | Type, address and database. Credentials are never read |

## Lineage

- Dashboard CONTAINS Chart
- DataSource FEEDS Chart, for every data source a panel or one of its queries uses
- Table FEEDS Chart and Table FEEDS Dashboard, for panels querying a PostgreSQL, MySQL, SQL Server or ClickHouse data source. Tables are named the way the native Marmot plugin for that database names them, so the edges land on assets discovered by those plugins. Template variables and Grafana macros in a query never produce an edge.

## Authentication

Create a service account in Grafana (Administration, Service accounts) and add a token to it. Tokens start with `glsa_`.

The Viewer role is enough: it reads dashboards, library panels and the data source list (names and addresses, never credentials). Should a token not be allowed to list data sources, the run still completes; charts then record the data source type and uid from the dashboard and no DataSource assets are created.

## Configuration

```yaml
host: "https://grafana.company.com"
api_key: "glsa_..."
include_panels: true
include_datasources: true
discover_lineage: true
page_size: 100
verify_ssl: true
```
