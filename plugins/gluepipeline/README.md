The Glue Pipelines plugin discovers AWS Glue workflows as Pipeline assets, their steps as Task assets, and the run history of the workflows, jobs and crawlers in the account.

It shares the `Glue` provider with the Glue plugin, so a workflow step links to the job or crawler asset that plugin already created rather than a copy of it. This plugin creates no Job, Crawler, Database or Bucket assets, only the lineage edges and the run history that point at them.

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
          "glue:ListWorkflows",
          "glue:BatchGetWorkflows",
          "glue:GetWorkflowRuns",
          "glue:ListJobs",
          "glue:BatchGetJobs",
          "glue:GetJobRuns",
          "glue:ListTriggers",
          "glue:BatchGetTriggers",
          "glue:GetCrawlers",
          "glue:ListCrawls",
          "glue:GetTags",
          "sts:GetCallerIdentity"
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
          "glue:ListWorkflows",
          "glue:BatchGetWorkflows",
          "glue:ListJobs",
          "glue:BatchGetJobs",
          "glue:ListTriggers",
          "glue:BatchGetTriggers"
        ],
        Resource: "*"
      }
    ]
  }}
/>

## AWS Configuration

See [AWS Configuration](./Shared%20Configuration/AWS%20Configuration.md) for the supported AWS configuration options.

## Tests

Unit tests need nothing. The end-to-end tests run against a Glue endpoint:

```
docker run -d --name marmot-test-gluepipeline -p 15560:5000 motoserver/moto:latest
MARMOT_TEST_GLUEPIPELINE_ENDPOINT=http://localhost:15560 go test ./...
```
