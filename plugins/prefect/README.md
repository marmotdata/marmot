The Prefect plugin discovers flows, their tasks and their recent runs from Prefect Cloud or a self-hosted Prefect server. It targets the Prefect 3 REST API.

Each flow becomes a Pipeline asset carrying its deployments, schedules and tags. Each distinct task of the flow's most recent run becomes a Task asset named after the task with the MRN `mrn://asset/task/prefect/<flow>.<task>`, linked to its flow by a CONTAINS edge and to the tasks it consumed by DEPENDS_ON edges. Recent flow runs are recorded as run history.

## Authentication

Prefect Cloud uses `api_key`, sent as a bearer token. A self-hosted server that has authentication enabled uses `auth_string`, a `user:password` pair sent as basic auth. A self-hosted server with no authentication needs neither.

The `host` field takes the API URL. Prefect serves its API under `/api`, which is added when the configured URL leaves it off. For Prefect Cloud, use the workspace URL: `https://api.prefect.cloud/api/accounts/<account>/workspaces/<workspace>`.

Set `zone` when you catalogue more than one Prefect server. It becomes part of every MRN, for example `mrn://asset/pipeline/prefect@prod/nightly-etl`.

## Table lineage

Prefect's Assets API reports the tables and buckets a flow run read and wrote, which the plugin turns into FEEDS and PRODUCES edges to the assets other plugins own. That API exists on Prefect Cloud only; a self-hosted server has no equivalent, so only the flow's own structure is discovered there.

Tables are named as their own plugin names them, for example `<database>.<schema>.<table>` for PostgreSQL and `<database>.<table>` for MySQL. When the asset key names a PostgreSQL, MySQL, MariaDB or ClickHouse server, the edge carries that host as a hint so Marmot can pick the right zone.
