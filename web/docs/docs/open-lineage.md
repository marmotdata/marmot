---
sidebar_position: 7
title: OpenLineage
description: Send OpenLineage events from Airflow, dbt, Spark, DuckDB and other producers to Marmot, and how Marmot files the jobs, datasets, runs and column lineage they describe.
---

import { TipBox } from '@site/src/components/Steps';

# OpenLineage

Marmot accepts [OpenLineage](https://openlineage.io/) events, so a tool that emits them, such as Airflow, dbt, Spark or DuckDB, adds its jobs, datasets and [lineage](lineage.md) to the catalog as it runs. Marmot follows OpenLineage 1.55.0 (core schema 2-0-2) and is tested against events captured from dbt-ol, openlineage-spark and DuckDB's duck_lineage extension. Report anything that doesn't map correctly on [GitHub](https://github.com/marmotdata/marmot/issues).

For your own integrations, send lineage with the [lineage API](lineage.md#send-lineage-from-your-own-pipelines) instead: it takes edges, down to the column, without the OpenLineage event format.

## What gets cataloged

Marmot takes all three OpenLineage event types:

- **RunEvent**: something ran. Marmot adds or updates the job and the datasets it read and wrote, links them with lineage and records the run in the job's run history.
- **JobEvent**: a job's lineage with no run behind it, such as a scheduler describing a pipeline it hasn't run yet. Marmot adds the job and its lineage and records no run.
- **DatasetEvent**: metadata about one dataset. Marmot adds or updates that dataset only.

Each input dataset is linked to the job, and the job to each output dataset. When an output carries column lineage, Marmot also adds a direct edge from each input dataset to the output with the column mappings and the job that moved the data. The **Lineage** tab draws that edge, and traces its columns, when the job isn't on the canvas; while the job is, selecting the input or output lists the mappings.

Lineage from OpenLineage accumulates: when a job stops reading a dataset, the edge from it stays until someone removes it on the **Lineage** tab.

A dataset Marmot hasn't seen before is added as a stub, so lineage is complete without filling search with undocumented assets. When the system's own plugin catalogs the dataset, the stub becomes that plugin's asset, keeping its id, its lineage and anything people added, and shows up in search.

### Runs

OpenLineage producers describe a run over several events, often with its inputs on one and its outputs on another, so Marmot combines every event with the same run id:

- A run that sent COMPLETE succeeded, and one that sent FAIL or ABORT failed. A failure stays a failure even if the producer sends COMPLETE after it, as Spark does.
- A run that has only sent START, RUNNING or OTHER is still running. Events can arrive in any order.
- An event Marmot already holds is ignored, so a producer that retries doesn't record a run twice.
- A failed run shows the message from its `errorMessage` facet, or from the `spark.exception` facet Spark sends instead.

## How datasets are named

OpenLineage names a dataset by a namespace, the address of the system that holds it, and a name, the path inside that system. Marmot turns those into the [MRN](MRNs/index.md) the system's own plugin uses, so the dataset an event mentions and the asset the plugin catalogs are one asset. The address becomes a host hint, which tells Marmot which [zone](MRNs/zones.md#point-other-plugins-at-the-right-zone) the dataset belongs to.

| Namespace | Name | Becomes |
| --- | --- | --- |
| `postgres://pg-eu:5432` | `analytics.public.orders` | `table/postgresql@<zone>/analytics.public.orders` |
| `mysql://db:3306` | `shop.orders` | `table/mysql@<zone>/shop.orders` |
| `mssql://sql:1433` or `sqlserver://sql:1433` | `shop.dbo.orders` | `table/sql-server@<zone>/shop.dbo.orders` |
| `snowflake://acme-prod` | `ANALYTICS.MARTS.ORDERS` | `table/snowflake@<zone>/analytics.marts.orders` |
| `bigquery` | `kelp-prod.sales.orders` | `table/bigquery/kelp-prod.sales.orders` |
| `awsathena://athena.eu-west-1.amazonaws.com` | `AwsDataCatalog.lake.orders` | `table/glue/lake.orders` |
| `arn:aws:glue:eu-west-1:123456789012` | `table/lake/orders` | `table/glue/lake.orders` |
| `kafka://broker:9092` | `orders.v1` | `topic/kafka@<zone>/orders.v1` |
| `s3://lake`, `gs://lake` | `raw/orders.parquet` | a File under the bucket, or a Bucket for a prefix |
| `abfss://raw@acct.dfs.core.windows.net` | `orders/2026.parquet` | a File in the `raw` container |
| `file` | `/data/out.parquet` | `file/file/-data-out.parquet` |
| `duckdb://warehouse/shop.duckdb` | `shop.main.orders` | `table/duckdb/shop.main.orders` |

Oracle, Db2, Teradata, Cassandra, Hive, Trino, Presto, ClickHouse, Redshift, Unity Catalog, HDFS, DBFS and Pub/Sub follow the same pattern. A namespace with no mapping, or one that isn't an address, keeps the naming Marmot 0.13 used.

When an event says a dataset's type in the `datasetType` facet, a VIEW becomes a View and so on. When it lists other names for the dataset in the `symlinks` facet, such as the Hive table over a set of files, every name leads to the same asset.

## Which facets are read

Marmot reads these facets. Every other facet, including custom ones, is accepted and kept in the run history.

| Facet | Where | What Marmot does with it |
| --- | --- | --- |
| `schema` | dataset | Sets the asset's columns, with the producer's types and descriptions |
| `columnLineage` | dataset | Column mappings on the direct edge from each input |
| `documentation` | dataset, job | Sets the description |
| `ownership` | dataset, job | Records the owners in metadata |
| `tags` | dataset, job | Adds the asset's tags, as `key:value`, leaving out tags on one column |
| `symlinks` | dataset | Makes each name lead to one asset |
| `dataSource`, `catalog` | dataset | Recorded in metadata, and used to find the dataset when the namespace isn't an address |
| `datasetType`, `storage`, `version` | dataset | Recorded in metadata; `datasetType` also sets the asset type |
| `lifecycleStateChange` | dataset | Recorded; a finished RENAME moves the asset and a finished DROP removes a stub |
| `dataQualityMetrics`, `inputStatistics`, `outputStatistics` | dataset | Row count, size and file count in metadata |
| `sql`, `sourceCode` | job | Sets the job's query and its language |
| `sourceCodeLocation`, `jobType` | job | Recorded in metadata; `jobType` also sets the asset type |
| `parent` | run | Links an Airflow DAG to its tasks and a dbt project run to its models |

A facet one event leaves out keeps what an earlier event said. Facets marked `_deleted` are ignored.

When a statement renames a dataset, Marmot moves the asset to the new name once the statement finishes, and the old MRN keeps leading to it. When a statement drops a dataset, Marmot removes the asset only if nothing but OpenLineage events ever mentioned it; an asset a plugin or a person wrote to is kept, marked as dropped, for its plugin's next run to remove.

## Authentication

Producers send events to `POST /api/v1/lineage` with an API key, either as `Authorization: Bearer <api-key>`, which the OpenLineage clients send, or as `X-API-Key: <api-key>`. Create a key under **Account → API keys** with **Create API key**, or use a service account's key.

To accept events without a key, turn authentication off for this endpoint:

```yaml
openlineage:
  auth:
    enabled: false
```

Or set `MARMOT_OPENLINEAGE_AUTH_ENABLED=false`.

<TipBox variant="warning" title="Anyone who can reach the endpoint can then write lineage">
Only turn authentication off on a network where every client is trusted.
</TipBox>

## Configure a producer

Point the producer at your Marmot URL. The OpenLineage clients add `/api/v1/lineage` themselves.

### Airflow

In `airflow.cfg`:

```ini
[openlineage]
transport = {"type": "http", "url": "https://marmot.example.com", "auth": {"type": "api_key", "apiKey": "<api-key>"}}
namespace = airflow-prod
```

### dbt

Run dbt through `dbt-ol` from the `openlineage-dbt` package, with the endpoint in the environment:

```bash
export OPENLINEAGE_URL=https://marmot.example.com
export OPENLINEAGE_API_KEY=<api-key>
export OPENLINEAGE_NAMESPACE=dbt-prod
dbt-ol run
```

### Spark

```bash
spark-submit \
  --packages io.openlineage:openlineage-spark_2.13:1.55.0 \
  --conf spark.extraListeners=io.openlineage.spark.agent.OpenLineageSparkListener \
  --conf spark.openlineage.transport.type=http \
  --conf spark.openlineage.transport.url=https://marmot.example.com \
  --conf spark.openlineage.transport.auth.type=api_key \
  --conf spark.openlineage.transport.auth.apiKey=<api-key> \
  --conf spark.openlineage.namespace=spark-prod \
  job.py
```

### DuckDB

Install DuckDB's `duck_lineage` community extension and give it the full endpoint, including `/api/v1/lineage`:

```sql
INSTALL duck_lineage FROM community;
LOAD duck_lineage;
SET duck_lineage_url = 'https://marmot.example.com/api/v1/lineage';
SET duck_lineage_api_key = '<api-key>';
```

duck_lineage files every table under its own namespace, `duckdb` by default. Marmot reads the `catalog` and `dataSource` facets it sends to find where each table lives: a table in a DuckDB file becomes the asset the DuckDB plugin catalogs, and a table in an attached Postgres or MySQL database becomes the asset the Postgres or MySQL plugin catalogs. Files it reads or writes are added as File assets.

duck_lineage 0.2 reports a statement that fails as COMPLETE, so DuckDB runs always show as succeeded.

The [OpenLineage documentation](https://openlineage.io/docs/) covers every producer and its options.

## Send events in batches

`POST /api/v1/lineage/batch` takes an array of OpenLineage events, as the OpenLineage batch endpoint does. Marmot processes them in order and answers with a summary that lists each event that failed and whether sending it again could help. Unlike the single-event endpoint, the batch endpoint always needs an API key, even with OpenLineage authentication turned off.

Both endpoints take gzip-compressed bodies sent with `Content-Encoding: gzip` and reject a body over 10 MiB once decompressed. A malformed event is answered with 400 and a message naming what's wrong, so the producer doesn't retry it.

## Upgrading from 0.13

Earlier releases named a job by the last part of its name, so `dag_a.load` and `dag_b.load` in one namespace were one asset. Jobs are now named by their full name. The first event for a job moves the asset it had before to the new name, keeping its id and run history, and the old MRN keeps leading to it.

Datasets in namespaces Marmot now maps, such as `abfss://` or `gs://`, and datasets duck_lineage sent, move the same way the first time an event mentions them.

## Next steps

- [Lineage](lineage.md) explains the graph and column lineage.
- [Zones](MRNs/zones.md) keep two systems of the same kind apart.
