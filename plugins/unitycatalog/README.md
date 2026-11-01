The Unity Catalog plugin discovers catalogs, tables, views, volumes, functions and registered models from a Unity Catalog server.

It talks to the open-source Unity Catalog REST API at `{host}/api/2.1/unity-catalog`, so it works against a self-hosted server and against anything else that serves the same API. Schemas are not assets of their own: every object is named by its three-part `catalog.schema.object` name and hangs off its catalog.

## Authentication

`token` is sent as a bearer token when it is set. A server running without authentication needs no token.

## Lineage

- Each catalog CONTAINS every table, view, volume, function and model under it.
- A view gets a VIEW_OF edge from each base table its definition reads, resolved against the view's own catalog and schema.
- A table with foreign key constraints gets a FOREIGN_KEY edge to each parent table discovered in the same run. The open-source server does not store constraints, so these come from Databricks-compatible servers only.
- A table or volume stored on cloud object storage gets a FEEDS edge from its bucket or container: `s3://` from the S3 plugin, `gs://` from GCS, `abfss://` from Azure Blob. The bucket asset itself belongs to those plugins; this plugin only emits the edge.
