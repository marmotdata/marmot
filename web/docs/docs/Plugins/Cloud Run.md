---
title: Cloud Run
description: Discovers services and jobs from Google Cloud Run, with lineage to the Cloud Storage buckets they mount.
status: experimental
---

# Cloud Run

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


The Cloud Run plugin discovers services and jobs from a Google Cloud project through the Cloud Run Admin API v2. Every service becomes a Service asset and every job a Job asset, named by region and id (`europe-west1/checkout-api`), because one id can exist in several regions of the same project.

Each asset records what the workload runs and how it is configured: the container image and ports, the service account, scaling limits, VPC access, the traffic split across revisions, and the labels on the resource. Container environment variables are recorded by name only, never by value.

Every region is scanned in one call unless `locations` is set. Regions Cloud Run reports as unreachable are logged and skipped rather than failing the run.

## Lineage

Cloud Run only reveals a data dependency through a volume mount, so that is the only relationship this plugin emits. A Cloud Storage volume becomes a `FEEDS` edge from the bucket to the service or job that mounts it, using the same identity the [Google Cloud Storage](./Google%20Cloud%20Storage.md) plugin gives that bucket, so the two plugins describe one asset.

Cloud SQL volumes are recorded in the `cloud_sql_instances` metadata field but produce no edge: the API does not say which engine an instance runs, so there is no provider to address it by. Secret and NFS volumes are metadata only.

## Run History

Each job reports its recent executions as run history. An execution that has started emits a `START`, then `COMPLETE`, `FAIL` or `ABORT` once it finishes, or `RUNNING` while it is still going. The task counters and the Cloud Logging link travel with each event. Set `include_executions` to `false` to skip the extra API call per job.

## Required Permissions

The service account needs the **Cloud Run Viewer** role (`roles/run.viewer`), or a custom role with `run.services.list`, `run.jobs.list` and `run.executions.list`.

## Example Configuration

```yaml

project_id: "acme-prod"
credentials_file: "/etc/marmot/cloudrun-service-account.json"
locations:
  - "europe-west1"
  - "us-central1"
include_jobs: true
include_executions: true
max_executions_per_job: 10
filter:
  include:
    - "^europe-west1/.*"
  exclude:
    - ".*-staging$"
tags:
  - "cloudrun"
  - "serverless"

```

## Configuration
The following configuration options are available:

| Property | Type | Required | Description |
|----------|------|----------|-------------|
| credentials_file | string | false | Path to service account JSON file |
| credentials_json | string | false | Service account JSON content |
| disable_auth | bool | false | Disable authentication, for local testing |
| endpoint | string | false | Custom endpoint URL, for testing against a local server |
| external_links | []ExternalLink | false | External links to show on all assets |
| filter | Filter | false | Filter discovered assets by name (regex) |
| include_executions | bool | false | Whether to read recent job executions as run history |
| include_jobs | bool | false | Whether to discover jobs |
| locations | []string | false | Regions to scan. Every region is scanned when this is empty |
| max_executions_per_job | int | false | How many recent executions to read per job |
| project_id | string | true | Google Cloud project ID |
| tags | TagsConfig | false | Tags to apply to discovered assets |

## Available Metadata

The following metadata fields are available:

| Field | Type | Description |
|-------|------|-------------|
| cloud_sql_instances | []string | Cloud SQL instances mounted as volumes, as project:region:instance |
| container_image | string | Image of the first container |
| container_images | []string | Images of every container in the revision or task |
| container_ports | []int64 | Ports the containers listen on |
| create_time | string | Creation timestamp |
| creator | string | Principal that created the resource |
| env_var_names | []string | Names of the container environment variables. Values are never recorded |
| execution_count | int64 | Number of executions created for the job |
| execution_environment | string | Sandbox generation the containers run in |
| gcs_volume_buckets | []string | Cloud Storage buckets mounted as volumes |
| generation | int64 | Number of times the configuration has changed |
| ingress | string | Which traffic is allowed to reach the service |
| `label_<key>` | string | One entry per label on the resource |
| last_modifier | string | Principal that last modified the resource |
| latest_created_execution | string | Id of the most recently created execution |
| latest_created_revision | string | Id of the most recently created revision |
| latest_ready_revision | string | Id of the most recent revision that became ready |
| launch_stage | string | Google Cloud launch stage of the features in use |
| location | string | Region the workload runs in |
| max_instance_count | int64 | Maximum number of instances the service scales to |
| max_instance_request_concurrency | int64 | Concurrent requests one instance accepts |
| max_retries | int64 | Retries allowed per failed task |
| min_instance_count | int64 | Minimum number of instances kept running |
| nfs_volumes | []string | NFS mounts, as server:path |
| parallelism | int64 | How many tasks may run at the same time |
| project_id | string | Google Cloud project the workload belongs to |
| ready | string | State of the terminal condition |
| reconciling | bool | Whether the resource is still converging on its desired state |
| secret_volumes | []string | Names of the Secret Manager secrets mounted as volumes. Values are never recorded |
| service_account | string | Service account the workload runs as |
| task_count | int64 | Number of tasks one execution runs |
| timeout | string | Maximum duration of a single request or task |
| traffic | string | Traffic split across revisions, for example latest=100 |
| uid | string | Server-assigned unique identifier |
| update_time | string | Last update timestamp |
| uri | string | HTTPS endpoint the service is served on |
| vpc_connector | string | Serverless VPC Access connector the workload uses |
| vpc_egress | string | Which outbound traffic is routed through the VPC |
