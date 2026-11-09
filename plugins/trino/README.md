The Trino plugin discovers all catalogs (connected data sources like PostgreSQL, Hive, Iceberg, S3, etc.), their schemas, and tables/views.

## Required Permissions

The connecting user needs `SELECT` access to `system.metadata.catalogs`, `system.metadata.table_comments`, and each catalog's `information_schema`. A read-only user with access to these system tables is sufficient.

## Catalog mapping and MRNs

Trino sits in front of other systems, and a catalog name says nothing about which server is behind it, so the plugin never guesses. Tables of a catalog that is not listed in `catalog_map` are Trino's own assets, named `<catalog>.<schema>.<table>` under the `Trino` provider, for example `mrn://asset/table/trino/warehouse_pg.public.orders`. A warning in the log names each unmapped catalog, because its tables won't merge with the native plugin's.

To make a catalog's tables the same assets the native plugin discovers, map it to the zone of that plugin and, for connectors whose native names start with a database, to the database:

```yaml
catalog_map:
  warehouse_pg:
    zone: prod-eu      # the zone option of the PostgreSQL plugin
    database: shop     # the database the catalog points at
  lake:
    zone: ""           # Hive or Iceberg catalogs need no database
```

A mapped catalog's tables get the MRN the native plugin gives them, for example `mrn://asset/table/postgresql@prod-eu/shop.public.orders`. `database` is required for PostgreSQL, SQL Server, Oracle, Redshift and Snowflake (the database) and BigQuery (the project); a catalog of one of those connectors without it is treated as unmapped. Query grants resolve tables the same way.

Set `zone` on the plugin itself to tell two Trino clusters apart (a name you choose, not a cloud availability zone); it becomes part of the MRN of the catalog and unmapped table assets.

## AI Enrichment

When your Trino instance has [AI functions](https://trino.io/docs/current/functions/ai.html) configured, the plugin can automatically enrich discovered assets:

- **Auto-generate descriptions** (`ai_generate_descriptions: true`) — Uses the AI connector's `ai_gen` function to produce one-sentence descriptions for tables that have no comment.
- **Auto-classify tables** (`ai_classify_tables: true`) — Uses the AI connector's `ai_classify` function to assign a category label (e.g., `analytics`, `pii`, `financial`) to each table, added as a tag like `ai-category:pii`.

### AI Setup

1. Configure an AI connector in your Trino installation (e.g., `ai.properties`)
2. Set `ai_catalog` to the catalog name of that connector
3. Enable `ai_generate_descriptions` and/or `ai_classify_tables`
4. Optionally customise `ai_classify_labels` and `ai_max_enrichments`

AI enrichment is best-effort - failures are logged as warnings but do not prevent normal discovery from completing.
