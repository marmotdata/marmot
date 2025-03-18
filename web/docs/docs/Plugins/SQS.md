---
title: SQS
description: This plugin discovers SQS queues from AWS accounts.
status: experimental
---

# SQS

This plugin discovers SQS queues from AWS accounts.

**Status:** experimental

## Example Configuration

```yaml

credentials:
  region: "us-west-2"
  profile: "default"
  # Optional: manual credentials
  id: ""
  secret: ""
  token: ""
  # Optional: role assumption
  role: ""
  role_external_id: ""
tags_to_metadata: true
include_tags:
  - "Environment"
  - "Team"
  - "Cost-Center"
tags:
  - "sqs"
  - "aws"
discover_dlq: true
filter:
  include:
    - "^prod-.*"
    - "^staging-.*"
  exclude:
    - ".*-test$"
    - ".*-dev$"

```

## Configuration
The following configuration options are available:

| Property | Type | Required | Description |
|----------|------|----------|-------------|
| discover_dlq | bool | false | Discover Dead Letter Queue relationships |

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