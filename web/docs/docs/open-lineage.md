---
title: OpenLineage
description: How Marmot turns OpenLineage events from Airflow, dbt, Spark, DuckDB and other producers into jobs, datasets, runs and column lineage, and lands each dataset on the asset its own plugin catalogs.
---

# OpenLineage

Tools that emit [OpenLineage](https://openlineage.io/) events, such as Airflow, dbt, Spark and DuckDB, can send them to Marmot as they run. Marmot adds each job and the datasets it read and wrote, links them with [lineage](lineage.md) down to the column, and records every run with its outcome. A producer always connects to Marmot, and Marmot never connects to the producer.

A dataset an event mentions lands on the asset the system's own plugin catalogs, so a Postgres table that dbt reads and the Postgres plugin finds is one asset, not two. To set a producer up, see [Send OpenLineage events to Marmot](Populating/send-openlineage-events.md); every accepted event type, namespace and facet is in the [OpenLineage reference](Populating/openlineage-reference.md).

For your own integrations, use the [lineage API](lineage.md#send-lineage-from-your-own-pipelines) instead. It takes edges, down to the column, without the OpenLineage event format.

## How it works

1. A producer sends an event to `POST /api/v1/lineage` each time a job starts, finishes or fails, or describes a job or dataset without a run.
2. Marmot works out which asset each dataset is from its namespace and name, the way the system's own plugin names it.
3. Marmot adds the job and any dataset it hasn't seen, or adds what the event says to the assets that exist.
4. Each input is linked to the job and the job to each output. Column lineage is stored on a direct edge from each input to the output.
5. Marmot records the event in the job's run history, combined with the run's other events.

## Jobs and datasets

Each OpenLineage job becomes a job asset named by its full job name, so `orders_etl.load` and `billing_etl.load` in one namespace are two jobs. A dbt model, a dbt test, an Airflow DAG and an Airflow task get their own asset types.

Events describe an asset a piece at a time, so a facet one event leaves out keeps what an earlier event said, and a facet marked `_deleted` is ignored. A dataset Marmot hasn't seen before is added as a stub. A stub keeps lineage complete without filling search with assets nobody has documented. When the system's plugin catalogs the dataset, the stub becomes that plugin's asset: it keeps its id, its lineage and anything people added, and shows up in search. What the plugin says about the asset, such as its schema and description, wins over what OpenLineage says.

## How datasets are named

OpenLineage names a dataset by a namespace, the address of the system that holds it, and a name, the path inside that system. Marmot turns the pair into the [MRN](MRNs/index.md) the system's plugin uses. The address becomes a host hint that tells Marmot which [zone](MRNs/zones.md#point-other-plugins-at-the-right-zone) the dataset belongs to, with the system's default port added when the address leaves it out:

| Namespace | Name | Becomes |
| --- | --- | --- |
| `postgres://pg-eu:5432` | `analytics.public.orders` | `table/postgresql@<zone>/analytics.public.orders` |
| `snowflake://acme-prod` | `ANALYTICS.MARTS.ORDERS` | `table/snowflake@<zone>/analytics.marts.orders` |
| `kafka://broker:9092` | `orders.v1` | `topic/kafka@<zone>/orders.v1` |

When the namespace isn't an address, Marmot looks at the `dataSource` and `catalog` facets for one. DuckDB's duck_lineage extension sends every table under the namespace `duckdb`, so a table in an attached Postgres database lands on the Postgres plugin's asset and a table in a DuckDB file lands on the DuckDB plugin's asset.

When an event lists other names for a dataset in the `symlinks` facet, such as the Hive table over a set of files, every name leads to the same asset.

## Runs

Producers describe a run over several events, often with its inputs on one and its outputs on another, so Marmot combines every event with the same run id:

- A run that sent COMPLETE succeeded, and a run that sent FAIL or ABORT failed. A failure stands even when the producer sends COMPLETE after it, as Spark does.
- A run that has sent only START, RUNNING or OTHER is still running. Events can arrive in any order.
- An event Marmot already holds is ignored, so a producer that retries doesn't record a run twice.
- A failed run shows the error from its `errorMessage` facet, or from the `spark.exception` facet Spark sends instead, in the job's **Run history**.

A run's `parent` facet names the run that started it. The job that recorded that run contains the child job, so a Spark application contains the jobs it ran, a dbt invocation its models and tests, and an Airflow DAG its tasks.

## Column lineage

When an output carries the `columnLineage` facet, Marmot stores which input columns feed each output column on a direct edge from each input to the output, naming the job that moved the data. A column that's copied unchanged carries no transformation; one that's aggregated, filtered or computed is marked as transformed.

On an asset's **Lineage** tab the data flows through the job, and the direct edge stays out of the way. Select the job to trace its columns: the tables either side open up and a line joins each source column to the column it feeds. The asset's side panel lists the same mappings, grouped by the tables they join.

## Renames and drops

The `lifecycleStateChange` facet says what a statement did to a dataset. Marmot records it on the asset, and acts on two of them once the statement has finished:

- **RENAME** moves the asset to its new name. It keeps its id, lineage and everything people added, and the old name keeps leading to it.
- **DROP** removes the asset if nothing but OpenLineage events ever mentioned it. An asset a plugin or a person wrote to is kept, marked as dropped, for its plugin's next run to remove.

A drop of a dataset Marmot has never seen adds nothing.

## Assets from earlier releases

Releases up to 0.13 named a job by the last part of its name, and named some datasets differently. The first event for a job or dataset moves the asset an earlier release made to its new name, keeping its id and run history, as long as nothing but OpenLineage wrote to it. The old MRN keeps leading to the asset.

## Limitations

- Lineage accumulates. When a job stops reading a dataset, the edge from it stays until someone removes it on the **Lineage** tab, because OpenLineage events never say that an input stopped being one.
- A namespace Marmot has no mapping for, such as `crate://` or `azurecosmos://`, keeps the naming Marmot 0.13 used, so its datasets don't line up with any plugin's assets.
- duck_lineage 0.2 reports a statement that fails as COMPLETE, so DuckDB runs always show as succeeded.
- openlineage-spark reports the jobs Spark runs to work out a file's schema with a START and nothing after, so they show as running. It also reports a `CREATE TABLE AS SELECT` under two job names, so the table has two jobs writing it.
- dbt-ol leaves seeds out of a model's inputs and sends no error message with a failed model. A seed reaches the models built on it through their column lineage only, and a failed model's run shows no error.
- Facets Marmot doesn't read, such as `dataQualityAssertions` and `nominalTime`, are kept in the run history but not shown.

## Next steps

- [Send OpenLineage events to Marmot](Populating/send-openlineage-events.md) from Airflow, dbt, Spark or DuckDB.
- [OpenLineage reference](Populating/openlineage-reference.md) lists every namespace, facet and response.
- [Lineage](lineage.md) explains the graph and column tracing.
