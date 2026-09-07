---
title: Kinesis
description: Discovers Kinesis Data Streams from AWS accounts.
status: experimental
---

# Kinesis

<div class="flex flex-col gap-3 mb-6 pb-6 border-b border-gray-200">
<div class="flex items-center gap-3">
<span class="inline-flex items-center rounded-full px-4 py-2 text-sm font-medium bg-earthy-yellow-300 text-earthy-yellow-900">Experimental</span>
</div>
<div class="flex items-center gap-2">
<span class="text-sm text-gray-500">Creates:</span>
<div class="flex flex-wrap gap-2"><span class="inline-flex items-center rounded-lg px-4 py-2 text-sm font-medium bg-earthy-green-100 text-earthy-green-800 border border-earthy-green-300">Assets</span></div>
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


The Kinesis plugin discovers Amazon Kinesis Data Streams across your AWS accounts. Each stream becomes a Stream asset carrying its capacity mode, retention, shard and consumer counts, encryption settings and tags, with a data preview that reads the oldest retained records from the first open shard.

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
          "kinesis:ListStreams",
          "kinesis:DescribeStreamSummary",
          "kinesis:ListShards",
          "kinesis:ListStreamConsumers",
          "kinesis:ListTagsForStream",
          "kinesis:GetShardIterator",
          "kinesis:GetRecords"
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
        Action: ["kinesis:ListStreams", "kinesis:DescribeStreamSummary"],
        Resource: "*"
      }
    ]
  }}
/>

`GetShardIterator` and `GetRecords` are only needed for the data preview. `ListShards`, `ListStreamConsumers` and `ListTagsForStream` can be left out when `include_shards`, `include_consumers` or `tags_to_metadata` are switched off; a denied call logs a warning and the stream is still catalogued.

## AWS Configuration

See [AWS Configuration](./Shared%20Configuration/AWS%20Configuration.md) for the supported AWS configuration options.



## Example Configuration

```yaml

credentials:
  region: "us-east-1"
  id: "<aws-secret-id>"
  secret: "<aws-secret-key>"
include_consumers: true
include_shards: true
tags:
  - "kinesis"

```

## Configuration
The following configuration options are available:

| Property | Type | Required | Description |
|----------|------|----------|-------------|
| credentials | AWSCredentials | false | AWS credentials configuration |
| external_links | []ExternalLink | false | External links to show on all assets |
| filter | Filter | false | Filter discovered assets by name (regex) |
| include_consumers | bool | false | Whether to list the enhanced fan-out consumers registered on each stream |
| include_shards | bool | false | Whether to list shards to count total and open shards per stream |
| include_tags | []string | false | List of AWS tags to include as metadata. By default, all tags are included. |
| tags | TagsConfig | false | Tags to apply to discovered assets |
| tags_to_metadata | bool | false | Convert AWS tags to Marmot metadata |

`include_consumers`, `include_shards` and `tags_to_metadata` default to true.

## Available Metadata

The following metadata fields are available:

| Field | Type | Description |
|-------|------|-------------|
| arn | string | Stream ARN |
| arrival_time | string | Approximate time the record reached the stream (RFC 3339) |
| consumer_count | int | Number of registered enhanced fan-out consumers |
| consumers | []string | Names of the registered enhanced fan-out consumers |
| created_at | string | Stream creation time (RFC 3339) |
| data | string | Record payload as text, or base64 when it is not valid UTF-8 |
| encryption_type | string | Server-side encryption type (NONE or KMS) |
| enhanced_monitoring | []string | Shard-level CloudWatch metrics that are enabled |
| kms_key_id | string | KMS key used for server-side encryption |
| open_shard_count | int | Number of shards still accepting writes |
| partition_key | string | Partition key the producer wrote the record with |
| region | string | AWS region the stream lives in |
| retention_hours | int | Record retention period in hours |
| sequence_number | string | Record sequence number within its shard |
| shard_count | int | Total number of shards, closed ones included |
| status | string | Stream status (CREATING, DELETING, ACTIVE, UPDATING) |
| stream_mode | string | Capacity mode (PROVISIONED or ON_DEMAND) |
| url | string | Link to the stream in the AWS console |
