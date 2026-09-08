---
title: Glue Pipelines
description: Discovers workflows, tasks and run history from AWS Glue.
status: experimental
---

# Glue Pipelines

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


The Glue Pipelines plugin catalogs AWS Glue workflows: one Pipeline asset per workflow, one Task asset per workflow step, and the run history of the workflows, jobs and crawlers in the account.

It shares the `Glue` provider with the [Glue](./Glue.md) plugin, so a workflow step that runs a job links to the job asset that plugin already created instead of a copy of it. Run the two together: this plugin never creates Job, Crawler, Database or Bucket assets, it only points at them.

## Lineage

- A workflow contains its steps.
- A step depends on the next step, from the workflow run graph.
- A job step depends on the Glue job it runs, and a crawler step on the crawler it runs.
- A trigger step depends on the jobs and crawlers it starts, and the jobs and crawlers it waits for feed into it.
- A crawler is fed by the S3 bucket of each of its targets and produces the Glue database it writes to.

## Run History

Run history is attached to the asset that ran: workflow runs to the workflow's Pipeline asset, job runs to the Glue plugin's Job asset, and crawls to its Crawler asset. Each run becomes a START event and a closing event: COMPLETE, FAIL, ABORT or RUNNING while it is still going.

Some Glue endpoints do not return the workflow run graph. When it is missing, the workflow's own triggers are used to describe its steps instead.

## Required Permissions

import { Collapsible } from "@site/src/components/Collapsible";

<Collapsible
  title="IAM Policy"
  icon="mdi:shield-check"
  policyJson={{
    Version: "2012-10-17",
    Statement: [
      {
        Effect: "Allow",
        Action: [
          "glue:ListWorkflows",
          "glue:BatchGetWorkflows",
          "glue:GetWorkflowRuns",
          "glue:ListJobs",
          "glue:BatchGetJobs",
          "glue:GetJobRuns",
          "glue:ListTriggers",
          "glue:BatchGetTriggers",
          "glue:GetCrawlers",
          "glue:ListCrawls",
          "glue:GetTags",
          "sts:GetCallerIdentity"
        ],
        Resource: "*"
      }
    ]
  }}
  minimalPolicyJson={{
    Version: "2012-10-17",
    Statement: [
      {
        Effect: "Allow",
        Action: [
          "glue:ListWorkflows",
          "glue:BatchGetWorkflows",
          "glue:ListJobs",
          "glue:BatchGetJobs",
          "glue:ListTriggers",
          "glue:BatchGetTriggers"
        ],
        Resource: "*"
      }
    ]
  }}
/>

## AWS Configuration

See [AWS Configuration](./Shared%20Configuration/AWS%20Configuration.md) for the supported AWS configuration options.

## Example Configuration

```yaml

credentials:
  region: "us-east-1"
  profile: "production"
  role: "<role>"
tags:
  - "aws"
include_workflows: true
include_triggers: true
include_run_history: true
run_history_limit: 20
include_crawlers: true
tags_to_metadata: true

```

## Configuration
The following configuration options are available:

| Property | Type | Required | Description |
|----------|------|----------|-------------|
| credentials | AWSCredentials | false | AWS credentials configuration |
| external_links | []ExternalLink | false | External links to show on all assets |
| filter | Filter | false | Filter discovered assets by name (regex) |
| include_crawlers | bool | false | Whether to read crawlers for lineage and runs |
| include_run_history | bool | false | Whether to collect workflow, job and crawler runs |
| include_tags | []string | false | List of AWS tags to include as metadata. By default, all tags are included. |
| include_triggers | bool | false | Whether to read trigger definitions |
| include_workflows | bool | false | Whether to discover Glue workflows |
| run_history_limit | int | false | How many recent runs to read per workflow, job and crawler |
| tags | TagsConfig | false | Tags to apply to discovered assets |
| tags_to_metadata | bool | false | Convert AWS tags to Marmot metadata |

## Available Metadata

The following metadata fields are available:

| Field | Type | Description |
|-------|------|-------------|
| attempt | int32 | Retry attempt number of a job run |
| crawler | string | Glue crawler the step runs |
| crawler_count | int | Number of crawler steps |
| created_on | string | When the workflow was created |
| default_run_properties | string | Run properties every run starts with, as key=value pairs |
| description | string | Workflow description |
| dpu_hour | float64 | DPU hours a crawl consumed |
| error_message | string | Error reported by a failed run |
| execution_time_seconds | int32 | How long a job run took |
| job | string | Glue job the step runs |
| job_count | int | Number of job steps |
| job_script_location | string | Location of the job script |
| job_type | string | Job command (glueetl, pythonshell, gluestreaming) |
| last_modified_on | string | When the workflow was last changed |
| last_run_completed | string | When the most recent run finished |
| last_run_id | string | Identifier of the most recent run |
| last_run_started | string | When the most recent run started |
| last_run_statistics | string | Action counters of the most recent run, as key=value pairs |
| last_run_status | string | Status of the most recent run |
| log_group | string | CloudWatch log group of a crawl |
| max_concurrent_runs | int32 | How many runs may overlap |
| node_count | int | Number of steps in the workflow |
| node_name | string | Step name inside the workflow |
| node_type | string | Step kind (job, crawler, trigger) |
| number_of_workers | int32 | Workers the job run used |
| region | string | AWS region the workflow lives in |
| state | string | Job or crawler run state reported by AWS |
| statistics | string | Action counters of a workflow run |
| status | string | Workflow run status reported by AWS |
| summary | string | What a crawl changed |
| trigger | string | Trigger that started the job run |
| trigger_actions | string | Jobs and crawlers the trigger starts |
| trigger_count | int | Number of trigger steps |
| trigger_predicate | string | What a conditional trigger waits for |
| trigger_schedule | string | Cron expression of a scheduled trigger |
| trigger_state | string | Trigger state reported by AWS |
| trigger_type | string | Trigger kind (SCHEDULED, CONDITIONAL, ON_DEMAND, EVENT) |
| unique_id | string | Identifier AWS gives the step in the run graph |
| url | string | AWS console link to the workflow |
| worker_type | string | Worker size the job run used |
| workflow | string | Workflow the step belongs to |
