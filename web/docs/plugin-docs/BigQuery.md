The BigQuery plugin discovers datasets, tables, and views from Google BigQuery projects. It captures schemas, statistics, and lineage relationships.

## Required Permissions

Assign `roles/bigquery.metadataViewer` to your service account, or these individual permissions:

- `bigquery.datasets.get`
- `bigquery.tables.get`
- `bigquery.tables.list`
