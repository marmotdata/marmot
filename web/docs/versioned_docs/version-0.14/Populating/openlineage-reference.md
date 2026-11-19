---
title: OpenLineage reference
description: The endpoints, event types, responses, namespaces and facets Marmot's OpenLineage support accepts, and what Marmot does with each.
---

# OpenLineage reference

Everything Marmot accepts from OpenLineage producers, following OpenLineage 1.55.0 (core schema 2-0-2). For how it fits together, see [OpenLineage](../open-lineage.md).

## Endpoints

| Endpoint | Takes | Authentication |
| --- | --- | --- |
| `POST /api/v1/lineage` | One event as JSON, optionally sent with `Content-Encoding: gzip` | `Authorization: Bearer <api-key>` or `X-API-Key: <api-key>`; can be turned off with `openlineage.auth.enabled: false` |
| `POST /api/v1/lineage/batch` | A JSON array of events, processed in order | Always an API key |

## Event types

| Event type | Recognised by | What Marmot files |
| --- | --- | --- |
| RunEvent | A `schemaURL` ending in `RunEvent`, or a `run` field | The job, its datasets, their lineage and the run |
| JobEvent | A `schemaURL` ending in `JobEvent`, or a `job` field and no `run` | The job, its datasets and their lineage, with no run |
| DatasetEvent | A `schemaURL` ending in `DatasetEvent`, or a `dataset` field and no `job` | The dataset |

## Event fields

| Field | Required on | When it's missing or unusual |
| --- | --- | --- |
| `eventTime` | Every event | A time without an offset is read as UTC |
| `run.runId` | RunEvent | Any string is accepted, not only a UUID |
| `job.namespace`, `job.name` | RunEvent, JobEvent | |
| `dataset.namespace`, `dataset.name` | DatasetEvent | |
| `namespace`, `name` of each input and output | Every event that lists them | |
| `eventType` | None | A missing or unknown type is read as `OTHER` |
| `producer`, `schemaURL` | None | The event type is read from the fields present |

## Responses

| Status | When |
| --- | --- |
| `200` | The event was filed. The batch endpoint answers with a summary of what failed |
| `400` | The body isn't JSON or the event is missing a required field; the message names it. Don't retry |
| `401`, `403` | The API key is missing, invalid or its owner can't manage assets |
| `413` | The body is over 10 MiB after decompression |
| `415` | `Content-Encoding` is something other than `gzip` |
| `500` | Marmot couldn't file the event. Retry |

## Namespaces

| Namespace | Name | Becomes |
| --- | --- | --- |
| `postgres://<host>:<port>` | `<database>.<schema>.<table>` | PostgreSQL table |
| `mysql://<host>:<port>`, `mariadb://` | `<database>.<table>` | MySQL or MariaDB table |
| `mssql://<host>:<port>`, `sqlserver://` | `<database>.<schema>.<table>` | SQL Server table |
| `oracle://`, `db2://`, `teradata://` | The path the system uses | Oracle, Db2 or Teradata table |
| `snowflake://<account>` | `<database>.<schema>.<table>` | Snowflake table |
| `redshift://<cluster>.<region>:<port>` | `<database>.<schema>.<table>` | Redshift table |
| `bigquery` | `<project>.<dataset>.<table>` | BigQuery table |
| `awsathena://athena.<region>.amazonaws.com` | `<catalog>.<database>.<table>` | Glue table, without the default `AwsDataCatalog` |
| `arn:aws:glue:<region>:<account>` | `table/<database>/<table>` | Glue table |
| `hive://`, `trino://`, `presto://`, `clickhouse://`, `cassandra://`, `unitycatalog://` | The path the system uses | A table of that system |
| `kafka://<host>:<port>` | `<topic>` | Kafka topic |
| `pubsub` | `topic:<project>:<topic>` or `subscription:<project>:<id>` | Pub/Sub topic or subscription |
| `s3://<bucket>`, `gs://<bucket>` | `<key>` | File for a data file, Bucket for a prefix |
| `abfss://<container>@<account>.dfs.core.windows.net`, `wasbs://` | `<path>` | File or container in Azure Blob Storage |
| `hdfs://<host>:<port>`, `dbfs://<workspace>` | `<path>` | HDFS or DBFS file or dataset |
| `file`, `file://<host>` | `<path>` | File, or a dataset for a directory |
| `duckdb://<path>` | `<database>.<schema>.<table>` | DuckDB table, as dbt-duckdb sends |
| A namespace that isn't an address | Any | The address in the `dataSource` or `catalog` facet, then the 0.13 naming |

## Facets

| Facet | On | What Marmot does with it |
| --- | --- | --- |
| `schema` | Dataset | Sets the asset's columns, in `ordinal_position` order, with the producer's types and descriptions |
| `columnLineage` | Dataset | Column mappings on a direct edge from each input to the output |
| `documentation` | Dataset, job | Sets the description |
| `ownership` | Dataset, job | `owner` and `owners` in metadata |
| `tags` | Dataset, job | Asset tags as `key:value`, leaving out tags on one column |
| `symlinks` | Dataset | Each name leads to one asset |
| `dataSource`, `catalog` | Dataset | Metadata, and the dataset's address when the namespace isn't one |
| `datasetType` | Dataset | Sets the asset type, such as View for `VIEW` |
| `storage`, `version` | Dataset | `storage_layer`, `file_format` and `version` in metadata |
| `lifecycleStateChange` | Dataset | `lifecycle_state` in metadata; a finished RENAME moves the asset and a finished DROP removes a stub |
| `dataQualityMetrics` | Dataset, input | `row_count`, `size_bytes` and `file_count` in metadata |
| `inputStatistics`, `outputStatistics` | Input, output | `input_` and `output_` row count, size and file count in metadata |
| `sql` | Job | Sets the job's query; `dialect` as `sql_dialect` |
| `sourceCode` | Job | Sets the job's query and its language |
| `sourceCodeLocation` | Job | `source_code_url`, repository, path, branch, tag and version in metadata |
| `jobType` | Job | Sets the asset type and provider; `processing_type` and `integration` in metadata |
| `parent`, `parentRun` | Run | The job that recorded the parent run contains this job |
| `errorMessage`, `spark.exception` | Run | The run's error in **Run history** |
| Any other facet | Any | Kept in the run history |
