The Lambda plugin discovers and catalogs AWS Lambda functions across your AWS accounts. It captures function metadata including runtime, memory, timeout, VPC configuration, layers, tracing, and tags.

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
          "lambda:ListFunctions",
          "lambda:GetFunction",
          "lambda:ListTags"
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
        Action: ["lambda:ListFunctions"],
        Resource: "*"
      }
    ]
  }}
/>

## AWS Configuration

See [AWS Configuration](./Shared%20Configuration/AWS%20Configuration.md) for the supported AWS configuration options.
