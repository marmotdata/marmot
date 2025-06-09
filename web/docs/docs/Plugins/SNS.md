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