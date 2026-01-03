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

## Features

### DAG Discovery
Discovers all DAGs as **Pipeline** assets with metadata including:
- Schedule interval and next run time
- Owners and file location
- Pause/active status
- Run history and success rate

### Task Discovery
Discovers tasks within DAGs as **Task** assets with:
- Operator type (PythonOperator, BashOperator, etc.)
- Trigger rules and retry configuration
- Task dependencies within the DAG
- Pool assignments

### Dataset Lineage (Airflow 2.4+)
Automatically tracks data flow using [Airflow Datasets](https://airflow.apache.org/docs/apache-airflow/stable/authoring-and-scheduling/datasets.html):
- **Input datasets** that trigger DAGs
- **Output datasets** produced by DAGs
- Cross-DAG dependencies through shared datasets

The plugin intelligently maps Dataset URIs to the correct asset types:
| URI Scheme | Asset Type | Example |
|------------|------------|---------|
| `s3://`, `s3a://` | S3 Bucket | `s3://my-bucket/data/` |
| `gs://`, `gcs://` | GCS Bucket | `gs://my-bucket/data/` |
| `kafka://` | Kafka Topic | `kafka://broker/events` |
| `postgresql://` | PostgreSQL Table | `postgresql://host/db/table` |
| `mysql://` | MySQL Table | `mysql://host/db/table` |
| `bigquery://` | BigQuery Table | `bigquery://project/dataset/table` |
| `snowflake://` | Snowflake Table | `snowflake://account/db/table` |

### Run History
Tracks DAG execution history with:
- Start and end times
- Run status (success, failed, running)
- Execution metrics and success rate

## Lineage

The Airflow plugin creates lineage edges showing the flow of data:

```
Input Dataset ──FEEDS──▶ DAG (Pipeline) ──PRODUCES──▶ Output Dataset
```

Example lineage from a typical ETL setup:
```
s3://raw-data ──▶ etl_pipeline ──▶ s3://processed-data ──▶ analytics_pipeline ──▶ s3://reports
```

Within DAGs, task dependencies are also tracked:
```
DAG ──CONTAINS──▶ Task
Task ──DEPENDS_ON──▶ Downstream Task
```

## Asset Types Created

| Asset Type | MRN Format | Description |
|------------|------------|-------------|
| Pipeline | `mrn://pipeline/airflow/{dag_id}` | Represents an Airflow DAG |
| Task | `mrn://task/airflow/{dag_id}.{task_id}` | Represents a task within a DAG |
| Bucket/Topic/Table | Varies by URI scheme | Datasets referenced by DAGs |

