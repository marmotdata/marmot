---
title: SNS
description: This plugin discovers SNS topics from AWS accounts.
status: experimental
---

# SNS

This plugin discovers SNS topics from AWS accounts.

**Status:** experimental

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
| aws | AWSConfig | false |  |
| credentials | AWSCredentials | false | AWS credentials configuration |
| external_links | []ExternalLink | false |  |
| filter | Filter | false | Filter patterns for AWS resources |
| global_documentation | []string | false |  |
| global_documentation_position | string | false |  |
| include_tags | []string | false | List of AWS tags to include as metadata |
| merge | MergeConfig | false |  |
| metadata | MetadataConfig | false |  |
| tags | TagsConfig | false |  |
| tags_to_metadata | bool | false | Convert AWS tags to Marmot metadata |

## Available Metadata

The following metadata fields are available:

| Field | Type | Description |
|-------|------|-------------|
| display_name | string | Display name of the topic |
| owner | string | AWS account ID that owns the topic |
| policy | string | Access policy of the topic |
| subscriptions_confirmed | string | Number of confirmed subscriptions |
| subscriptions_pending | string | Number of pending subscriptions |
| tags | map[string]string | AWS resource tags |
| topic_arn | string | The ARN of the SNS topic |