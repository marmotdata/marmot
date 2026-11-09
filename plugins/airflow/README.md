The Airflow plugin ingests metadata from Apache Airflow, including DAGs (Directed Acyclic Graphs), tasks, and dataset lineage. It connects to Airflow's REST API to discover your orchestration layer and track data dependencies through Airflow's native Dataset feature.

## Prerequisites

- **Airflow 2.0+** for basic DAG and task discovery
- **Airflow 2.4+** for Dataset-based lineage tracking
- REST API enabled with authentication configured

:::tip[Authentication]
The plugin supports two authentication methods:

- **Basic Auth**: Username and password
- **API Token**: For token-based authentication

Configure authentication in your Airflow instance via `airflow.cfg`:

```ini
[api]
auth_backends = airflow.api.auth.backend.basic_auth
```

:::

## Naming

A DAG is named by its `dag_id` and a task by its DAG and task id, for example `mrn://asset/task/airflow/etl.load`. Set `zone` when you catalogue more than one Airflow, for example `mrn://asset/pipeline/airflow@prod/etl`.

Dataset URIs are turned into the asset the owning plugin catalogues. PostgreSQL and Redshift URIs (`postgres://host:5432/<database>/<schema>/<table>`) become `<database>.<schema>.<table>`, MySQL (`mysql://host/<database>/<table>`) becomes `<database>.<table>`, and BigQuery (`bigquery://<project>/<dataset>/<table>` or `bigquery:<project>.<dataset>.<table>`) becomes `<project>.<dataset>.<table>`. S3 and GCS stay at the bucket and Kafka at the topic. The host in the URI is passed on so Marmot can pick the zone that owns it.
