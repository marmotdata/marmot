---
title: Send OpenLineage events to Marmot
description: Point Airflow, dbt, Spark or DuckDB at Marmot so their jobs, datasets, runs and column lineage appear in the catalog as they run.
---

import { Tabs, TabPanel, TipBox } from '@site/src/components/Steps';

# Send OpenLineage events to Marmot

Point a tool that emits OpenLineage events at Marmot, and its jobs, datasets and lineage appear in the catalog as it runs. For what Marmot does with the events, see [OpenLineage](../open-lineage.md).

## Before you start

- You have a Marmot API key. Create one under **Account** → **API keys** with **Create API key**, or use a service account's key.
- The machine the producer runs on can reach Marmot over HTTPS.
- The key is in the producer's environment as `MARMOT_API_KEY`:

```bash
export MARMOT_API_KEY=<api-key>
```

## Point the producer at Marmot

Give the producer your Marmot URL and the API key. The OpenLineage clients send events to `/api/v1/lineage` on the URL you give them, with the key as a bearer token:

<Tabs items={[
{ label: "Airflow", value: "airflow" },
{ label: "dbt", value: "dbt" },
{ label: "Spark", value: "spark" },
{ label: "DuckDB", value: "duckdb" }
]}>

<TabPanel value="airflow">

Set these in the environment of every Airflow scheduler and worker, with the `apache-airflow-providers-openlineage` package installed:

```bash
export OPENLINEAGE_URL=https://marmot.example.com
export OPENLINEAGE_API_KEY="${MARMOT_API_KEY}"
export AIRFLOW__OPENLINEAGE__NAMESPACE=airflow-prod
```

</TabPanel>

<TabPanel value="dbt">

Install `openlineage-dbt` next to dbt and run dbt through `dbt-ol`, with the endpoint in the environment:

```bash
export OPENLINEAGE_URL=https://marmot.example.com
export OPENLINEAGE_API_KEY="${MARMOT_API_KEY}"
export OPENLINEAGE_NAMESPACE=dbt-prod
```

Then run dbt as usual:

```bash
dbt-ol run
```

</TabPanel>

<TabPanel value="spark">

Add the OpenLineage listener when you submit the job. Match the package's Scala version, `_2.13` here, to your Spark build:

```bash
spark-submit \
  --packages io.openlineage:openlineage-spark_2.13:1.55.0 \
  --conf spark.extraListeners=io.openlineage.spark.agent.OpenLineageSparkListener \
  --conf spark.openlineage.transport.type=http \
  --conf spark.openlineage.transport.url=https://marmot.example.com \
  --conf spark.openlineage.transport.auth.type=api_key \
  --conf spark.openlineage.transport.auth.apiKey="${MARMOT_API_KEY}" \
  --conf spark.openlineage.namespace=spark-prod \
  orders_job.py
```

</TabPanel>

<TabPanel value="duckdb">

Load DuckDB's `duck_lineage` community extension at the start of each session. Unlike the other clients, it takes the full endpoint, including `/api/v1/lineage`:

```sql
INSTALL duck_lineage FROM community;
LOAD duck_lineage;
SET duck_lineage_url = 'https://marmot.example.com/api/v1/lineage';
SET duck_lineage_api_key = getenv('MARMOT_API_KEY');
```

</TabPanel>

</Tabs>

Each producer's own documentation covers its other options; the [OpenLineage documentation](https://openlineage.io/docs/) links to them all.

## Optional: accept events without a key

When every client on the network is trusted, you can turn authentication off for `POST /api/v1/lineage`. Add this to the Marmot server config:

```yaml
openlineage:
  auth:
    enabled: false
```

Or set `MARMOT_OPENLINEAGE_AUTH_ENABLED=false`. The batch endpoint, `POST /api/v1/lineage/batch`, still needs a key.

<TipBox variant="warning" title="Anyone who can reach the endpoint can then write lineage">
Only turn authentication off on a network where every client is trusted.
</TipBox>

## Check that it worked

1. Run a job in the producer.
2. In Marmot, search **Discover** for the last part of the job's name, such as `load` for `orders_etl.load`, and open the job.
3. Select the **Lineage** tab. The job's inputs feed into it and its outputs come out of it.
4. Select the **Run history** tab. The run shows as **Succeeded**, **Failed** or **Running**; select a run to see its full error.

## Troubleshooting

| What you see | What to do |
| --- | --- |
| The producer logs `401 Unauthorized` | Check that `MARMOT_API_KEY` holds a key that hasn't expired or been revoked. |
| The producer logs `403` with `Permission denied` | The key's owner needs a role that can manage assets. |
| The producer logs `400` with `Invalid OpenLineage event: run.runId is required`, or another required field | The producer sent an event Marmot can't file. Upgrade the producer's OpenLineage package. |
| The producer logs `413` with `Request body is over the 10 MiB limit` | Turn on the client's gzip compression, or turn off large facets such as Spark's logical plan. |
| A table appears twice, once from the producer and once from its plugin | The producer's namespace isn't an address Marmot recognises, or the two are in different zones. Compare the namespace with the [OpenLineage reference](openlineage-reference.md) and [point the producer at the right zone](../MRNs/zones.md#point-other-plugins-at-the-right-zone). |
| A DuckDB statement failed but its run shows **Succeeded** | duck_lineage 0.2 reports failed statements as COMPLETE. Check DuckDB's own output for the error. |

## Next steps

- [OpenLineage](../open-lineage.md) explains how Marmot names datasets, combines runs and traces columns.
- [OpenLineage reference](openlineage-reference.md) lists every namespace, facet and response.
- [Zones](../MRNs/zones.md) keep two systems of the same kind apart.
