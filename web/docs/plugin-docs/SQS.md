The SQS plugin discovers and catalogs Amazon SQS queues across your AWS accounts. It captures queue configurations and can discover Dead Letter Queue relationships.

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
