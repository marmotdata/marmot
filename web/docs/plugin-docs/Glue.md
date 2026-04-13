The Glue plugin discovers and catalogs AWS Glue resources including jobs, databases, tables and crawlers. It captures metadata such as job configurations, table schemas, crawler schedules and database properties. Iceberg-managed tables are automatically skipped (use the dedicated Iceberg plugin instead).

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
          "glue:GetJobs",
          "glue:GetDatabases",
          "glue:GetTables",
          "glue:GetCrawlers",
          "glue:GetTags"
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
        Action: [
          "glue:GetJobs",
          "glue:GetDatabases",
          "glue:GetTables",
          "glue:GetCrawlers"
        ],
        Resource: "*"
      }
    ]
  }}
/>

## AWS Configuration

See [AWS Configuration](./Shared%20Configuration/AWS%20Configuration.md) for the supported AWS configuration options.
