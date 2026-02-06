---
title: SQS
description: This plugin discovers SQS queues from AWS accounts.
status: experimental
---

# SQS

<div class="flex flex-col gap-3 mb-6 pb-6 border-b border-gray-200">
<div class="flex items-center gap-3">
<span class="inline-flex items-center rounded-full px-4 py-2 text-sm font-medium bg-earthy-yellow-300 text-earthy-yellow-900">Experimental</span>
</div>
<div class="flex items-center gap-2">
<span class="text-sm text-gray-500">Creates:</span>
<div class="flex flex-wrap gap-2"><span class="inline-flex items-center rounded-lg px-4 py-2 text-sm font-medium bg-earthy-green-100 text-earthy-green-800 border border-earthy-green-300">Assets</span><span class="inline-flex items-center rounded-lg px-4 py-2 text-sm font-medium bg-earthy-green-100 text-earthy-green-800 border border-earthy-green-300">Lineage</span></div>
</div>
</div>

import { CalloutCard } from '@site/src/components/DocCard';

<CalloutCard
  title="Configure in the UI"
  description="This plugin can be configured directly in the Marmot UI with a step-by-step wizard."
  href="/docs/Populating/UI"
  buttonText="View Guide"
  variant="secondary"
  icon="mdi:cursor-default-click"
/>


The SQS plugin discovers and catalogs Amazon SQS queues across your AWS accounts. It captures queue configurations and can discover Dead Letter Queue relationships.

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
          "sqs:ListQueues",
          "sqs:GetQueueAttributes",
          "sqs:ListQueueTags"
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
        Action: ["sqs:ListQueues", "sqs:GetQueueAttributes"],
        Resource: "*"
      }
    ]
  }}
/>



## Example Configuration

```yaml

credentials:
  region: "us-east-1" 
  id: "<aws-secret-id>"
  secret: "<aws-secret-key>"
tags:
  - "sns"

```

## Configuration
The following configuration options are available:

| Property | Type | Required | Description |
|----------|------|----------|-------------|
| credentials | AWSCredentials | false | AWS credentials configuration |
| discover_dlq | bool | false | Discover Dead Letter Queue relationships |
| external_links | []ExternalLink | false | External links to show on all assets |
| filter | Filter | false | Filter discovered assets by name (regex) |
| include_tags | []string | false | List of AWS tags to include as metadata. By default, all tags are included. |
| tags | TagsConfig | false | Tags to apply to discovered assets |
| tags_to_metadata | bool | false | Convert AWS tags to Marmot metadata |

## Available Metadata

The following metadata fields are available:

| Field | Type | Description |
|-------|------|-------------|
| content_based_deduplication | bool | Whether content-based deduplication is enabled |
| deduplication_scope | string | Deduplication scope for FIFO queues |
| delay_seconds | string | Delay seconds for messages |
| fifo_queue | bool | Whether this is a FIFO queue |
| fifo_throughput_limit | string | FIFO throughput limit type |
| maximum_message_size | string | Maximum message size in bytes |
| message_retention_period | string | Message retention period in seconds |
| queue_arn | string | The ARN of the SQS queue |
| receive_message_wait_time | string | Long polling wait time in seconds |
| redrive_policy | string | Redrive policy JSON string |
| tags | map[string]string | AWS resource tags |
| visibility_timeout | string | The visibility timeout for the queue |