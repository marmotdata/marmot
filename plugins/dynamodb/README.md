The DynamoDB plugin discovers and catalogs Amazon DynamoDB tables across your AWS accounts. It captures table metadata including key schema, billing mode, indexes, encryption settings, TTL, point-in-time recovery, streams, and tags.

## Required Permissions

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

## Keyless authentication

On Marmot Cloud or Marmot Enterprise the pipeline can present its own identity instead of access keys. Register your Marmot instance as an IAM OIDC identity provider (client id `sts.amazonaws.com`), create a role whose trust policy allows `sts:AssumeRoleWithWebIdentity` for that provider with `<issuer host>:sub` equal to the pipeline's subject, `pipeline:<name>` as reported by the pipeline API, and set `credentials.role_arn` and `credentials.region`. No key exists anywhere; Marmot mints a short-lived token for each run.
