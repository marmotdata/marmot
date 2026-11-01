The Metabase plugin discovers dashboards, questions, metrics and models from a Metabase instance, and links them to the warehouse tables they read.

## Assets

| Metabase object | Marmot type | Name |
|-----------------|-------------|------|
| Dashboard | Dashboard | `<collection path>/<dashboard name>` |
| Question, metric | Chart | `<collection path>/<card name>` |
| Model | Data Model Object | `<collection path>/<card name>` |

The collection path is the names of the collections from the root down, joined with `/`, without the root collection itself. Items in the root collection keep their bare name. Numeric ids stay in metadata.

Native (SQL) cards carry their query. Every asset links back to Metabase.

## Lineage

- Dashboard CONTAINS every card placed on it.
- Table FEEDS card, and Table FEEDS every dashboard showing the card. Query builder cards name their source table and joins by id; native cards are scanned for the tables after `FROM` and `JOIN`, with `[[optional clauses]]`, CTEs, subqueries and comments ignored, and matched against the tables Metabase has synced for the card's database.
- Model or question FEEDS the cards built on it, including `{{#id}}` references in native SQL.

Tables are addressed exactly as the warehouse's own Marmot plugin addresses them (for example `mrn://table/postgresql/orders` for a Postgres table), so the edges land on the assets that plugin creates. No table assets are created by this plugin; until the warehouse plugin has run, the server drops those edges.

| Metabase engine | Provider | Table name |
|-----------------|----------|------------|
| postgres, mysql, mariadb, bigquery-cloud-sdk, clickhouse, sqlite | PostgreSQL, MySQL, MariaDB, BigQuery, ClickHouse, SQLite | bare table name |
| athena | Glue | bare table name |
| mongo | MongoDB (type Collection) | bare collection name |
| snowflake, redshift, sqlserver | Snowflake, Redshift, SQL Server | `database.schema.table` |
| presto-jdbc, starburst, databricks | Trino, Databricks | `catalog.schema.table` |
| oracle, druid | Oracle, Druid | `schema.table` |
| anything else (h2, ...) | engine name, first letter upper-cased | `schema.table` |

## Authentication

Set `api_key` (Metabase 0.49 and newer, created under Settings, Authentication, API keys; the key needs read access to the collections and databases to discover). Without one the plugin logs in with `username` and `password` and uses the session for the run.

## Supported Versions

Tested against Metabase 0.63. Both the current `stages` query shape (0.57 and newer) and the older `type: native | query` shape are read, and `ordered_cards` is accepted in place of `dashcards` on servers before 0.48.

## Example Configuration

```yaml
host: "https://metabase.example.com"
api_key: "${METABASE_API_KEY}"
include_charts: true
include_models: true
discover_lineage: true
include_archived: false
```
