---
title: SQS
description: This plugin discovers SQS queues from AWS accounts.
status: experimental
---

# SQS

**Status:** experimental

The SQS plugin discovers and catalogs Amazon SQS queues across your AWS accounts. It captures queue configurations, attributes, and can optionally discover Dead Letter Queue relationships between queues.

## Prerequisites

### AWS Permissions

The plugin requires the following IAM permissions:

```json
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Action": [
        "sqs:ListQueues",
        "sqs:GetQueueAttributes",
        "sqs:ListQueueTags"
      ],
      "Resource": "*"
    }
  ]
}
```

### Minimal Permissions

For basic queue discovery without tags:

```json
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Action": ["sqs:ListQueues", "sqs:GetQueueAttributes"],
      "Resource": "*"
    }
  ]
}
```


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
| filter | Filter | false | Filter patterns for AWS resources |
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