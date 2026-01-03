---
title: Airflow
description: Ingests metadata from Apache Airflow including DAGs, tasks, and dataset lineage.
status: experimental
---

# Airflow

<div class="flex flex-col gap-3 mb-6 pb-6 border-b border-gray-200">
<div class="flex items-center gap-3">
<span class="inline-flex items-center rounded-full px-4 py-2 text-sm font-medium bg-earthy-yellow-300 text-earthy-yellow-900">Experimental</span>
</div>
<div class="flex items-center gap-2">
<span class="text-sm text-gray-500">Creates:</span>
<div class="flex flex-wrap gap-2"><span class="inline-flex items-center rounded-lg px-4 py-2 text-sm font-medium bg-earthy-green-100 text-earthy-green-800 border border-earthy-green-300">Assets</span><span class="inline-flex items-center rounded-lg px-4 py-2 text-sm font-medium bg-earthy-green-100 text-earthy-green-800 border border-earthy-green-300">Lineage</span><span class="inline-flex items-center rounded-lg px-4 py-2 text-sm font-medium bg-earthy-green-100 text-earthy-green-800 border border-earthy-green-300">Run History</span></div>
</div>
</div>


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



## Example Configuration

```yaml

host: "http://localhost:8080"
username: "admin"
password: "${AIRFLOW_PASSWORD}"
discover_dags: true
discover_tasks: true
discover_datasets: true
include_run_history: true
run_history_days: 7
only_active: true
dag_filter:
  include:
    - "^analytics_.*"
  exclude:
    - ".*_test$"
tags:
  - "airflow"
  - "orchestration"

```

## Configuration
The following configuration options are available:

| Property | Type | Required | Description |
|----------|------|----------|-------------|
| api_token | string | false | API token for authentication (alternative to basic auth) |
| dag_filter | plugin.Filter | false | Filter DAGs by ID pattern (include/exclude regex) |
| discover_dags | bool | false | Discover Airflow DAGs as Pipeline assets |
| discover_datasets | bool | false | Discover Airflow Datasets for lineage (requires Airflow 2.4+) |
| discover_tasks | bool | false | Discover tasks within DAGs |
| external_links | []ExternalLink | false | External links to show on all assets |
| host | string | false | Airflow webserver URL (e.g., http://localhost:8080) |
| include_run_history | bool | false | Include DAG run history in metadata |
| only_active | bool | false | Only discover active (unpaused) DAGs |
| password | string | false | Password for basic authentication |
| run_history_days | int | false | Number of days of run history to fetch |
| tags | TagsConfig | false | Tags to apply to discovered assets |
| username | string | false | Username for basic authentication |

## Available Metadata

The following metadata fields are available:

| Field | Type | Description |
|-------|------|-------------|
| consumer_count | int | Number of DAGs that consume this dataset |
| created_at | string | Dataset creation timestamp |
| dag_id | string | Unique DAG identifier |
| dag_id | string | Parent DAG ID |
| dag_run_id | string | Unique identifier for the DAG run |
| description | string | DAG description |
| downstream_tasks | []string | List of downstream task IDs |
| end_date | string | End time of the run |
| execution_date | string | Logical execution date |
| file_path | string | Path to DAG definition file |
| is_active | bool | Whether DAG is active |
| is_paused | bool | Whether DAG is paused |
| last_parsed_time | string | Last time the DAG file was parsed |
| last_run_date | string | Execution date of the last DAG run |
| last_run_id | string | ID of the last DAG run |
| last_run_state | string | State of the last DAG run (success, failed, running) |
| next_run_date | string | Next scheduled run date |
| operator_name | string | Airflow operator class name (e.g., BashOperator, PythonOperator) |
| owners | string | DAG owners (comma-separated) |
| pool | string | Execution pool for the task |
| producer_count | int | Number of tasks that produce this dataset |
| retries | int | Number of retries configured for the task |
| run_count | int | Number of runs in the lookback period |
| run_type | string | Type of run (scheduled, manual, backfill) |
| schedule_interval | string | DAG schedule (cron expression or preset) |
| start_date | string | Actual start time of the run |
| state | string | Run state (queued, running, success, failed) |
| success_rate | float64 | Success rate percentage over the lookback period |
| task_id | string | Task identifier within the DAG |
| trigger_rule | string | Task trigger rule (e.g., all_success, one_success) |
| updated_at | string | Dataset last update timestamp |
| uri | string | Dataset URI identifier |