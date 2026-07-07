# marmot-plugin-airflow

Marmot plugin for Apache Airflow. Ingests metadata from Airflow's REST API, including DAGs (Directed Acyclic Graphs), tasks, and dataset lineage, to discover your orchestration layer and track data dependencies through Airflow's native Dataset feature.

Marmot plugins are standalone binaries that the Marmot host launches on demand via [go-plugin](https://github.com/hashicorp/go-plugin) and talks to over gRPC. It is built on the [Marmot plugin SDK](https://github.com/marmotdata/plugin-sdk).

## Prerequisites

- **Airflow 2.0+** for basic DAG and task discovery
- **Airflow 2.4+** for Dataset-based lineage tracking
- REST API enabled with authentication configured

## Authentication

The plugin supports two authentication methods:

- **Basic Auth**: Username and password
- **API Token**: For token-based authentication

Configure authentication in your Airflow instance via `airflow.cfg`:

```ini
[api]
auth_backends = airflow.api.auth.backend.basic_auth
```

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
filter:
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
| `host` | string | true | Airflow webserver URL (e.g., http://localhost:8080) |
| `username` | string | false | Username for basic authentication |
| `password` | string | false | Password for basic authentication (sensitive) |
| `api_token` | string | false | API token for authentication (alternative to basic auth, sensitive) |
| `discover_dags` | bool | false | Discover Airflow DAGs as Pipeline assets (default `true`) |
| `discover_tasks` | bool | false | Discover tasks within DAGs (default `true`) |
| `discover_datasets` | bool | false | Discover Airflow Datasets for lineage, requires Airflow 2.4+ (default `true`) |
| `include_run_history` | bool | false | Include DAG run history in metadata (default `true`) |
| `run_history_days` | int | false | Number of days of run history to fetch (default `7`) |
| `only_active` | bool | false | Only discover active (unpaused) DAGs (default `true`) |
| `external_links` | []ExternalLink | false | External links to show on all assets |
| `filter` | Filter | false | Filter discovered assets by name (regex) |
| `tags` | TagsConfig | false | Tags to apply to discovered assets |

Either `username`/`password` or `api_token` must be provided.

## Lineage

The plugin creates lineage edges alongside the discovered assets:

- `CONTAINS` from a DAG to each of its tasks
- `DEPENDS_ON` between tasks, following downstream task relationships
- `FEEDS` from a Dataset to each DAG that consumes it
- `PRODUCES` from a DAG to each Dataset its tasks produce

Dataset URIs are mapped to the matching Marmot asset type where possible (for example `s3://` becomes an S3 Bucket, `kafka://` a Kafka Topic, `postgresql://` a PostgreSQL Table), so lineage connects to assets discovered by other plugins.

## Available Metadata

DAG (Pipeline) assets carry the following fields:

| Field | Type | Description |
|-------|------|-------------|
| `dag_id` | string | Unique DAG identifier |
| `description` | string | DAG description |
| `file_path` | string | Path to DAG definition file |
| `schedule_interval` | string | DAG schedule (cron expression or preset) |
| `is_paused` | bool | Whether DAG is paused |
| `is_active` | bool | Whether DAG is active |
| `owners` | string | DAG owners (comma-separated) |
| `last_run_state` | string | State of the last DAG run (success, failed, running) |
| `last_run_id` | string | ID of the last DAG run |
| `last_run_date` | string | Execution date of the last DAG run |
| `next_run_date` | string | Next scheduled run date |
| `last_parsed_time` | string | Last time the DAG file was parsed |
| `success_rate` | float64 | Success rate percentage over the lookback period |
| `run_count` | int | Number of runs in the lookback period |

Task assets carry the following fields:

| Field | Type | Description |
|-------|------|-------------|
| `task_id` | string | Task identifier within the DAG |
| `dag_id` | string | Parent DAG ID |
| `operator_name` | string | Airflow operator class name (e.g., BashOperator, PythonOperator) |
| `trigger_rule` | string | Task trigger rule (e.g., all_success, one_success) |
| `retries` | int | Number of retries configured for the task |
| `pool` | string | Execution pool for the task |
| `downstream_tasks` | []string | List of downstream task IDs |

Dataset assets carry the following fields:

| Field | Type | Description |
|-------|------|-------------|
| `uri` | string | Dataset URI identifier |
| `created_at` | string | Dataset creation timestamp |
| `updated_at` | string | Dataset last update timestamp |
| `producer_count` | int | Number of tasks that produce this dataset |
| `consumer_count` | int | Number of DAGs that consume this dataset |

DAG run history events carry the following run facets:

| Field | Type | Description |
|-------|------|-------------|
| `dag_run_id` | string | Unique identifier for the DAG run |
| `state` | string | Run state (queued, running, success, failed) |
| `execution_date` | string | Logical execution date |
| `start_date` | string | Actual start time of the run |
| `end_date` | string | End time of the run |
| `run_type` | string | Type of run (scheduled, manual, backfill) |

## Development

Build and test:

```sh
make build
make test
```

To run a local build inside Marmot:

```sh
make install
```

This copies the binary to `~/.marmot/plugins/`, the directory Marmot scans for local plugins. A local plugin shadows the released core plugin with the same name: Marmot skips downloading it and loads your build instead. Delete the binary from `~/.marmot/plugins/` to fall back to the released version.

If your Marmot runs with a custom plugins directory (`MARMOT_PLUGINS_DIR`), set the same value for `make install` so both point at the same place.
