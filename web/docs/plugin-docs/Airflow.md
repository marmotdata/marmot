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
