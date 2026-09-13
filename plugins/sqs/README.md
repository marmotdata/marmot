The SQS plugin discovers and catalogs Amazon SQS queues across your AWS accounts. It captures queue configurations and can discover Dead Letter Queue relationships.

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
          "sqs:ListQueues",
          "sqs:GetQueueAttributes",
          "sqs:ListQueueTags"
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
        Action: ["sqs:ListQueues", "sqs:GetQueueAttributes"],
        Resource: "*"
      }
    ]
  }}
/>

## AWS Configuration

See [AWS Configuration](./Shared%20Configuration/AWS%20Configuration.md) for the supported AWS configuration options.

## Keyless authentication

On Marmot Cloud or Marmot Enterprise the pipeline can present its own identity instead of access keys. Register your Marmot instance as an IAM OIDC identity provider (client id `sts.amazonaws.com`), create a role whose trust policy allows `sts:AssumeRoleWithWebIdentity` for that provider with `<issuer host>:sub` equal to the pipeline's subject, `pipeline:<name>` as reported by the pipeline API, and set `credentials.role_arn` and `credentials.region`. No key exists anywhere; Marmot mints a short-lived token for each run.
