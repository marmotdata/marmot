The Flink plugin discovers jobs from an Apache Flink JobManager. Each job becomes a Pipeline, each vertex of its job graph a Task, with CONTAINS edges from the Pipeline to its Tasks and DEPENDS_ON edges between Tasks taken from the job plan. The timestamps Flink keeps for a job's state changes are recorded as run history.

It reads the JobManager REST API (`/config`, `/jobs/overview`, `/jobs/{jid}`, `/jobs/{jid}/config`, `/jobs/{jid}/exceptions`), which needs no credentials. `username`/`password` and `token` are for a proxy placed in front of the JobManager.

## Naming

A Pipeline is named after the job. Flink lets several jobs share a name, in which case the most recently started job keeps the bare name and the others are named `<name> (<jid>)`. A Task is named `<pipeline name>/<vertex name>`.

## Jobs the JobManager still lists

The JobManager keeps listing finished, failed and cancelled jobs until it restarts. They are discovered by default; set `include_completed: false` to keep only jobs that are still running.
