The Trino plugin discovers all catalogs (connected data sources like PostgreSQL, Hive, Iceberg, S3, etc.), their schemas, and tables/views.

## Required Permissions

The connecting user needs `SELECT` access to `system.metadata.catalogs`, `system.metadata.table_comments`, and each catalog's `information_schema`. A read-only user with access to these system tables is sufficient.

## AI Enrichment

When your Trino instance has [AI functions](https://trino.io/docs/current/functions/ai.html) configured, the plugin can automatically enrich discovered assets:

- **Auto-generate descriptions** (`ai_generate_descriptions: true`) — Uses the AI connector's `ai_gen` function to produce one-sentence descriptions for tables that have no comment.
- **Auto-classify tables** (`ai_classify_tables: true`) — Uses the AI connector's `ai_classify` function to assign a category label (e.g., `analytics`, `pii`, `financial`) to each table, added as a tag like `ai-category:pii`.

### AI Setup

1. Configure an AI connector in your Trino installation (e.g., `ai.properties`)
2. Set `ai_catalog` to the catalog name of that connector
3. Enable `ai_generate_descriptions` and/or `ai_classify_tables`
4. Optionally customise `ai_classify_labels` and `ai_max_enrichments`

AI enrichment is best-effort - failures are logged as warnings but do not prevent normal discovery from completing.re logged as warnings but do not prevent normal discovery from completing.
