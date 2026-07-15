---
title: AWS Configuration
description: Shared AWS credentials and options used by Marmot's AWS-backed plugins.
sidebar_position: 2
---

# AWS Configuration

AWS-backed plugins such as [S3](../S3.md), [SQS](../SQS.md), [SNS](../SNS.md), [Glue](../Glue.md), [Lambda](../Lambda.md), [DynamoDB](../DynamoDB.md) share common credentials configuration. The same config is used by the S3 backend in [File Sources](./File%20Sources.md).

## Credentials

Credentials are configured under `credentials`:

```yaml
credentials:
  use_default: true
  region: "us-east-1"
```

| Field              | Description                                                                                    |
| ------------------ | ---------------------------------------------------------------------------------------------- |
| `use_default`      | Use the default AWS credential chain (env vars, shared config, instance profile). Recommended. |
| `id`               | AWS access key ID.                                                                             |
| `secret`           | AWS secret access key.                                                                         |
| `token`            | AWS session token.                                                                             |
| `profile`          | Profile name from the shared credentials file.                                                 |
| `role`             | IAM role ARN to assume.                                                                        |
| `role_external_id` | External ID for cross-account role assumption.                                                 |
| `region`           | AWS region.                                                                                    |
| `endpoint`         | Custom endpoint URL (useful for LocalStack or other S3-compatible services).                   |

## Default credential chain

The recommended option. Marmot will pick up credentials from environment variables, `~/.aws/credentials`, `~/.aws/config`, container credentials or EC2/EKS instance profiles in the usual order.

```yaml
credentials:
  use_default: true
  region: "us-east-1"
```

## Static credentials

Provide an access key pair directly. Use a session `token` for temporary credentials.

```yaml
credentials:
  id: "AKIA..."
  secret: "..."
  region: "us-east-1"
```

## Named profile

Use a named profile from `~/.aws/credentials` or `~/.aws/config`.

```yaml
credentials:
  profile: "my-profile"
  region: "us-east-1"
```

## Assume role

Assume an IAM role after loading the base credentials. `role_external_id` is supported for cross-account assumption.

```yaml
credentials:
  use_default: true
  role: "arn:aws:iam::123456789012:role/MarmotReader"
  role_external_id: "optional-external-id"
  region: "us-east-1"
```

## Custom endpoint

Point at a non-AWS endpoint such as LocalStack.

```yaml
credentials:
  use_default: true
  region: "us-east-1"
  endpoint: "http://localhost:4566"
```
