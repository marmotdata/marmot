---
title: Glue
description: This plugin discovers jobs, databases, tables and crawlers from AWS Glue.
status: experimental
---

# Glue

<div class="flex flex-col gap-3 mb-6 pb-6 border-b border-gray-200">
<div class="flex items-center gap-3">
<span class="inline-flex items-center rounded-full px-4 py-2 text-sm font-medium bg-earthy-yellow-300 text-earthy-yellow-900">Experimental</span>
</div>
<div class="flex items-center gap-2">
<span class="text-sm text-gray-500">Creates:</span>
<div class="flex flex-wrap gap-2"><span class="inline-flex items-center rounded-lg px-4 py-2 text-sm font-medium bg-earthy-green-100 text-earthy-green-800 border border-earthy-green-300">Assets</span></div>
</div>
</div>

import { CalloutCard } from '@site/src/components/DocCard';

<CalloutCard
  title="Configure in the UI"
  description="This plugin can be configured directly in the Marmot UI with a step-by-step wizard."
  href="/docs/Populating/UI"
  buttonText="View Guide"
  variant="secondary"
  icon="mdi:cursor-default-click"
/>


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



## Example Configuration

```yaml

credentials:
  region: "us-east-1"
  profile: "production"
  role: "<role>"
tags:
  - "aws"
discover_jobs: true
discover_databases: true
discover_tables: true
discover_crawlers: true

```

## Configuration
The following configuration options are available:

| Property | Type | Required | Description |
|----------|------|----------|-------------|
| credentials | AWSCredentials | false | AWS credentials configuration |
| discover_crawlers | bool | false | Whether to discover Glue crawlers |
| discover_databases | bool | false | Whether to discover Glue databases |
| discover_jobs | bool | false | Whether to discover Glue jobs |
| discover_tables | bool | false | Whether to discover Glue tables |
| external_links | []ExternalLink | false | External links to show on all assets |
| filter | Filter | false | Filter discovered assets by name (regex) |
| include_tags | []string | false | List of AWS tags to include as metadata. By default, all tags are included. |
| tags | TagsConfig | false | Tags to apply to discovered assets |
| tags_to_metadata | bool | false | Convert AWS tags to Marmot metadata |

## Available Metadata

The following metadata fields are available:

| Field | Type | Description |
|-------|------|-------------|
| catalog_id | string | ID of the Data Catalog |
| classification | string | Classification of the table data (csv, parquet, json, etc.) |
| classifiers | string | Custom classifiers used by the crawler |
| connections | string | Connections used by the job |
| create_time | string | Date and time the database was created |
| create_time | string | Date and time the table was created |
| created_on | string | Date and time the job was created |
| creation_time | string | Date and time the crawler was created |
| database_name | string | Target database for the crawler |
| database_name | string | Name of the database containing the table |
| description | string | Description of the database |
| glue_version | string | Glue version used by the job |
| input_format | string | Hadoop input format class |
| last_crawl_error | string | Error message from the last crawl |
| last_crawl_status | string | Status of the last crawl |
| last_crawl_time | string | Start time of the last crawl |
| last_modified_on | string | Date and time the job was last modified |
| last_updated | string | Date and time the crawler was last updated |
| location | string | S3 location of the table data |
| location_uri | string | Location of the database |
| max_capacity | float64 | Maximum number of DPU that can be allocated |
| max_retries | int | Maximum number of retries |
| number_of_workers | int32 | Number of workers allocated to the job |
| output_format | string | Hadoop output format class |
| owner | string | Owner of the table |
| parameters | string | Database parameters |
| partition_keys | string | Partition key columns |
| recrawl_behavior | string | Recrawl behavior policy |
| retention | int32 | Retention period in days |
| role | string | IAM role ARN assigned to the job |
| role | string | IAM role ARN assigned to the crawler |
| schedule | string | Cron schedule expression |
| schema_delete_behavior | string | Behavior when schema objects are deleted |
| schema_update_behavior | string | Behavior when schema changes are detected |
| script_location | string | S3 location of the job script |
| security_configuration | string | Security configuration applied to the job |
| serde | string | Serialization/deserialization library |
| state | string | Current state of the crawler (READY, RUNNING, STOPPING) |
| table_type | string | Type of table (EXTERNAL_TABLE, VIRTUAL_VIEW, etc.) |
| targets | string | Summary of crawler targets |
| timeout | int32 | Job timeout in minutes |
| type | string | Job command type (glueetl, pythonshell, gluestreaming) |
| update_time | string | Date and time the table was last updated |
| worker_type | string | Worker type (Standard, G.1X, G.2X, etc.) |