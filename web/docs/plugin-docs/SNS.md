The SNS plugin discovers and catalogs Amazon SNS topics across your AWS accounts. It captures topic configurations, subscription details, and tags.

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
          "sns:ListTopics",
          "sns:GetTopicAttributes",
          "sns:ListTagsForResource"
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
        Action: ["sns:ListTopics", "sns:GetTopicAttributes"],
        Resource: "*"
      }
    ]
  }}
/>
