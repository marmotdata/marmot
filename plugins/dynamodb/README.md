---
title: DynamoDB
description: This plugin discovers DynamoDB tables from AWS accounts.
status: experimental
---

# DynamoDB

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
  href="/docs/Populating/UI"
  buttonText="View Guide"
  variant="secondary"
  icon="mdi:cursor-default-click"
/>


The DynamoDB plugin discovers and catalogs Amazon DynamoDB tables across your AWS accounts. It captures table metadata including key schema, billing mode, indexes, encryption settings, TTL, point-in-time recovery, streams, and tags.

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
          "dynamodb:ListTables",
          "dynamodb:DescribeTable",
          "dynamodb:DescribeTimeToLive",
          "dynamodb:DescribeContinuousBackups",
          "dynamodb:ListTagsOfResource"
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
        Action: ["dynamodb:ListTables", "dynamodb:DescribeTable"],
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

```

## Configuration
The following configuration options are available:

| Property | Type | Required | Description |
|----------|------|----------|-------------|
| credentials | AWSCredentials | false | AWS credentials configuration |
| external_links | []ExternalLink | false | External links to show on all assets |
| filter | Filter | false | Filter discovered assets by name (regex) |
| include_tags | []string | false | List of AWS tags to include as metadata. By default, all tags are included. |
| tags | TagsConfig | false | Tags to apply to discovered assets |
| tags_to_metadata | bool | false | Convert AWS tags to Marmot metadata |

## Available Metadata

The following metadata fields are available:

| Field | Type | Description |
|-------|------|-------------|
| attribute_definitions | string | Attribute definitions for the table's key schema |
| billing_mode | string | Billing mode of the table (PROVISIONED or PAY_PER_REQUEST) |
| continuous_backups | string | Continuous backups status (ENABLED or DISABLED) |
| creation_date | string | Date and time when the table was created |
| deletion_protection | string | Whether deletion protection is enabled |
| encryption_status | string | Status of server-side encryption |
| encryption_type | string | Type of server-side encryption (AES256 or KMS) |
| global_table_replicas | string | Regions where global table replicas exist |
| gsi_count | int | Number of global secondary indexes |
| item_count | int64 | Number of items in the table |
| key_schema | string | Key schema of the table (partition and sort keys) |
| lsi_count | int | Number of local secondary indexes |
| pitr_status | string | Point-in-time recovery status (ENABLED or DISABLED) |
| read_capacity_units | int64 | Provisioned read capacity units |
| stream_enabled | string | Whether DynamoDB Streams is enabled |
| stream_view_type | string | Stream view type (KEYS_ONLY, NEW_IMAGE, OLD_IMAGE, NEW_AND_OLD_IMAGES) |
| table_arn | string | The ARN of the DynamoDB table |
| table_class | string | Table class (STANDARD or STANDARD_INFREQUENT_ACCESS) |
| table_size_bytes | int64 | Total size of the table in bytes |
| table_status | string | Current status of the table (ACTIVE, CREATING, etc.) |
| tags | map[string]string | AWS resource tags |
| ttl_attribute | string | Attribute name used for Time to Live |
| ttl_status | string | Time to Live status (ENABLED or DISABLED) |
| write_capacity_units | int64 | Provisioned write capacity units |