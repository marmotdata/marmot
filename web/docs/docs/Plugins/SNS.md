---
title: SNS
description: This plugin discovers SNS topics from AWS accounts.
status: experimental
---

# SNS

**Status:** experimental

The SNS plugin discovers and catalogs Amazon SNS topics across your AWS accounts. It captures topic configurations, subscription details, access policies, and AWS resource tags.

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
        "sns:ListTopics",
        "sns:GetTopicAttributes",
        "sns:ListTagsForResource"
      ],
      "Resource": "*"
    }
  ]
}
```

### Minimal Permissions

For basic topic discovery without tags:

```json
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Action": ["sns:ListTopics", "sns:GetTopicAttributes"],
      "Resource": "*"
    }
  ]
}
```


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
| filter | Filter | false | Filter patterns for AWS resources |
| include_tags | []string | false | List of AWS tags to include as metadata. By default, all tags are included. |
| tags | TagsConfig | false | Tags to apply to discovered assets |
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