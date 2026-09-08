---
title: Prefect
description: Discovers flows, tasks and run history from Prefect Cloud or a self-hosted Prefect server.
status: experimental
---

# Prefect

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


The Prefect plugin discovers flows, their tasks and their recent runs from Prefect Cloud or a self-hosted Prefect server. It targets the Prefect 3 REST API.

Each flow becomes a Pipeline asset carrying its deployments, schedules and tags, and links back to the flow in the Prefect UI. Each distinct task of the flow's most recent run becomes a Task asset named `<flow>/<task>`, joined to its flow by a CONTAINS edge and to the tasks it consumed by DEPENDS_ON edges. Recent flow runs are recorded as run history.

Prefect names a task `<function name>-<hash of its source>`. The hash changes whenever the task's code changes, so the asset name drops it and the full key is kept in the `task_key` metadata field.

## Authentication

Prefect Cloud uses `api_key`, sent as a bearer token. A self-hosted server with authentication enabled uses `auth_string`, a `user:password` pair sent as basic auth. A self-hosted server with no authentication needs neither.

The `host` field takes the API URL. Prefect serves its API under `/api`, which is added when the configured URL leaves it off. For Prefect Cloud, use the workspace URL: `https://api.prefect.cloud/api/accounts/<account>/workspaces/<workspace>`.

## Table Lineage

Prefect's Assets API reports the tables and buckets a flow run read and wrote. The plugin turns those into FEEDS edges from the sources into the Pipeline and PRODUCES edges from the Pipeline to what it wrote, using the names the owning plugin gives them. That API exists on Prefect Cloud only; a self-hosted server has no equivalent, so only the flow's own structure is discovered there.

## Example Configuration

```yaml

host: "http://localhost:4200/api"
auth_string: "admin:password"
include_tasks: true
include_deployments: true
include_run_history: true
run_history_limit: 10
filter:
  include:
    - "^nightly.*"
  exclude:
    - ".*-test$"
tags:
  - "prefect"
  - "orchestration"

```

## Configuration
The following configuration options are available:

| Property | Type | Required | Description |
|----------|------|----------|-------------|
| api_key | string | false | Prefect Cloud API key |
| auth_string | string | false | Self-hosted server credentials, as user:password |
| external_links | []ExternalLink | false | External links to show on all assets |
| filter | Filter | false | Filter discovered assets by name (regex) |
| host | string | true | Prefect API URL, for example http://localhost:4200/api |
| include_deployments | bool | false | Read deployments for schedules, tags and descriptions |
| include_run_history | bool | false | Record recent flow runs as run history |
| include_tasks | bool | false | Discover the tasks of each flow's most recent run |
| run_history_limit | int | false | How many recent runs to read per flow |
| tags | TagsConfig | false | Tags to apply to discovered assets |
| verify_ssl | bool | false | Check the server's TLS certificate |

## Available Metadata

The following metadata fields are available:

| Field | Type | Description |
|-------|------|-------------|
| created | string | When the flow was first seen by Prefect |
| deployment | string | Deployment that started the run, when it was not started by hand |
| deployment_count | int | Number of deployments of this flow |
| deployments | []string | Deployment names, newest first |
| entrypoint | string | File and function the newest deployment runs |
| flow | string | Name of the flow the task belongs to |
| flow_id | string | Prefect's internal identifier for the flow |
| labels | map[string]string | Labels Prefect attached to the flow |
| last_run_at | string | Start time of the most recent run |
| last_run_state | string | State of the most recent run (COMPLETED, FAILED, RUNNING) |
| last_state | string | State of the task in the most recent run (COMPLETED, FAILED) |
| parameter_count | int | Number of parameters the run was given |
| paused | bool | Whether the newest deployment is paused |
| run_count | int | Number of recent runs read, or how many times a task ran |
| run_name | string | Name Prefect generated for the run |
| schedules | []string | Active schedules as cron expressions, intervals or recurrence rules |
| state_name | string | Prefect's display name for the run's state |
| success_rate | float64 | Percentage of the recent runs that completed |
| tags | []string | Tags from the flow and its deployments |
| task_key | string | Prefect's task key, the task name plus a hash of its source |
| total_run_time | float64 | Seconds the run spent running |
| updated | string | When the flow was last updated |
| url | string | Address of the flow in the Prefect UI |
| work_pools | []string | Work pools the deployments run on |
