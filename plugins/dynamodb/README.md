# marmot-plugin-dynamodb

Marmot plugin for [Amazon DynamoDB](https://aws.amazon.com/dynamodb/). Lists the DynamoDB tables in an AWS account and produces a `Table` asset per table with its identity (ARN, status, creation date), key schema and attribute definitions, billing mode and provisioned throughput, index counts, encryption settings, streams configuration, TTL, continuous backups / point-in-time recovery status, global table replicas, and optionally AWS resource tags.

Authentication uses the standard AWS credential chain (environment variables, shared config, EC2/ECS metadata) with optional IAM role assumption.

## Example Configurations

### Default credentials

```yaml
credentials:
  region: "us-east-1"
tags:
  - "aws"
  - "dynamodb"
```

### Named profile

```yaml
credentials:
  region: "us-west-2"
  profile: "production"
```

### Assume role

```yaml
credentials:
  region: "eu-west-1"
  role: "arn:aws:iam::123456789012:role/MarmotDiscovery"
  role_external_id: "${MARMOT_EXTERNAL_ID}"
tags_to_metadata: true
include_tags:
  - "Team"
  - "Environment"
```

### Filtering by name

```yaml
credentials:
  region: "us-east-1"
filter:
  include:
    - "^prod-.*"
  exclude:
    - ".*-temp$"
```

## Required Permissions

Minimal IAM policy for discovering tables and their full metadata:

```json
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Action": [
        "dynamodb:ListTables",
        "dynamodb:DescribeTable",
        "dynamodb:DescribeTimeToLive",
        "dynamodb:DescribeContinuousBackups",
        "dynamodb:ListTagsOfResource"
      ],
      "Resource": "*"
    }
  ]
}
```

For a name-only listing, `dynamodb:ListTables` and `dynamodb:DescribeTable` are sufficient.

## Development

Build and test:

```sh
make build
make test
```

To run a local build inside Marmot:

```sh
make install
```

This copies the binary to `~/.marmot/plugins/`, the directory Marmot scans for local plugins. A local plugin shadows the released core plugin with the same name: Marmot skips downloading it and loads your build instead. Delete the binary from `~/.marmot/plugins/` to fall back to the released version.

If your Marmot runs with a custom plugins directory (`MARMOT_PLUGINS_DIR`), set the same value for `make install` so both point at the same place.
