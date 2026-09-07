---
title: Flink
description: Discovers jobs and their vertices from an Apache Flink JobManager.
status: experimental
---

# Flink

<div class="flex flex-col gap-3 mb-6 pb-6 border-b border-gray-200">
<div class="flex items-center gap-3">
<span class="inline-flex items-center rounded-full px-4 py-2 text-sm font-medium bg-earthy-yellow-300 text-earthy-yellow-900">Experimental</span>
</div>
<div class="flex items-center gap-2">
<span class="text-sm text-gray-500">Creates:</span>
<div class="flex flex-wrap gap-2"><span class="inline-flex items-center rounded-lg px-4 py-2 text-sm font-medium bg-earthy-green-100 text-earthy-green-800 border border-earthy-green-300">Assets</span><span class="inline-flex items-center rounded-lg px-4 py-2 text-sm font-medium bg-earthy-green-100 text-earthy-green-800 border border-earthy-green-300">Lineage</span><span class="inline-flex items-center rounded-lg px-4 py-2 text-sm font-medium bg-earthy-green-100 text-earthy-green-800 border border-earthy-green-300">Run History</span></div>
</div>
</div>

import { CalloutCard } from '@site/src/components/DocCard';

<CalloutCard
  title="Configure in the UI"
  description="This plugin can be configured directly in the Marmot UI with a step-by-step wizard."
  docId="Populating/UI"
  buttonText="View Guide"
  variant="secondary"
  icon="mdi:cursor-default-click"
/>


The Flink plugin discovers jobs from an Apache Flink JobManager through its REST API. Each job becomes a Pipeline and each vertex of its job graph a Task, linked by CONTAINS edges from the Pipeline and DEPENDS_ON edges between Tasks taken from the job plan. The timestamps Flink keeps for a job's state changes (created, running, finished, failed, cancelled) are recorded as run history on the Pipeline.

The JobManager REST API needs no credentials. `username`/`password` and `token` are only for a proxy placed in front of it.

## Naming

A Pipeline is named after the job. Flink lets several jobs share a name, in which case the most recently started job keeps the bare name and the others are named `<name> (<jid>)`. A Task is named `<pipeline name>/<vertex name>`.

## Jobs the JobManager still lists

The JobManager keeps listing finished, failed and cancelled jobs until it restarts. They are discovered by default; set `include_completed: false` to keep only jobs that are still running.



## Example Configuration

```yaml

host: "http://flink-jobmanager.internal:8081"
include_tasks: true
include_run_history: true
include_completed: true
filter:
  include:
    - "^etl_.*"
  exclude:
    - ".*_test$"
tags:
  - "flink"
  - "streaming"

```

## Configuration
The following configuration options are available:

| Property | Type | Required | Description |
|----------|------|----------|-------------|
| external_links | []ExternalLink | false | External links to show on all assets |
| filter | Filter | false | Filter discovered assets by name (regex) |
| host | string | true | JobManager REST URL, for example http://localhost:8081 |
| include_completed | bool | false | Include the finished, failed and cancelled jobs the JobManager still lists |
| include_run_history | bool | false | Record each job's state changes as run history |
| include_tasks | bool | false | Discover each job vertex as a Task asset |
| password | string | false | Password for basic auth |
| tags | TagsConfig | false | Tags to apply to discovered assets |
| token | string | false | Bearer token, when a proxy in front of the JobManager requires it |
| username | string | false | Username for basic auth, when a proxy in front of the JobManager requires it |
| verify_ssl | bool | false | Verify the JobManager's TLS certificate |

## Available Metadata

The following metadata fields are available:

| Field | Type | Description |
|-------|------|-------------|
| description | string | Operator chain from the job plan |
| duration_ms | int64 | Job or vertex duration in milliseconds |
| end_time | string | When the job or vertex reached its final state (RFC3339), absent while it runs |
| error | string | Root cause of a failed job, trimmed to 500 characters |
| execution_mode | string | Execution mode (Flink 1.x only) |
| flink_version | string | Version of the Flink cluster |
| is_stoppable | bool | Whether the job can be stopped with a savepoint |
| jid | string | Flink job id |
| max_parallelism | int | Configured maximum parallelism, absent when unset |
| operator | string | Operator name from the job plan, when Flink reports one |
| parallelism | int | Job parallelism from the execution config, or vertex parallelism |
| pipeline | string | Name of the Pipeline the vertex belongs to |
| read_bytes | int64 | Bytes read by the vertex |
| read_records | int64 | Records read by the vertex |
| restart_strategy | string | Restart strategy description |
| start_time | string | When the job was submitted or the vertex started (RFC3339) |
| state | string | Job state (RUNNING, FINISHED, FAILED, CANCELED, ...) |
| status | string | Vertex status (RUNNING, FINISHED, FAILED, CANCELED, ...) |
| task_counts | map[string]int | Number of tasks per state (running, finished, failed, ...) |
| url | string | Job page in the Flink web UI |
| vertex_count | int | Number of vertices in the job graph |
| vertex_id | string | Vertex id within the job graph |
| write_bytes | int64 | Bytes written by the vertex |
| write_records | int64 | Records written by the vertex |
