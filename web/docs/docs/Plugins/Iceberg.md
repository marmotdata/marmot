---
title: Iceberg
description: This plugin discovers namespaces, tables and views from Iceberg catalogs (REST and AWS Glue).
status: experimental
---

# Iceberg

<div class="flex flex-col gap-3 mb-6 pb-6 border-b border-gray-200">
<div class="flex items-center gap-3">
<span class="inline-flex items-center rounded-full px-4 py-2 text-sm font-medium bg-earthy-yellow-300 text-earthy-yellow-900">Experimental</span>
</div>
<div class="flex items-center gap-2">
<span class="text-sm text-gray-500">Creates:</span>
<div class="flex flex-wrap gap-2"><span class="inline-flex items-center rounded-lg px-4 py-2 text-sm font-medium bg-earthy-green-100 text-earthy-green-800 border border-earthy-green-300">Assets</span><span class="inline-flex items-center rounded-lg px-4 py-2 text-sm font-medium bg-earthy-green-100 text-earthy-green-800 border border-earthy-green-300">Lineage</span></div>
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


The Iceberg plugin discovers namespaces, tables and views from Iceberg catalogs. It supports both REST catalogs and AWS Glue Data Catalog as backends.

## AWS Glue Catalog Permissions

When using `catalog_type: "glue"`, the following IAM permissions are required:

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
          "glue:GetDatabases",
          "glue:GetDatabase",
          "glue:GetTables",
          "glue:GetTable"
        ],
        Resource: "*"
      },
      {
        Effect: "Allow",
        Action: [
          "s3:GetObject"
        ],
        Resource: "arn:aws:s3:::*/*",
        Condition: {
          StringLike: {
            "s3:prefix": "*/metadata/*"
          }
        }
      }
    ]
  }}
  minimalPolicyJson={{
    Version: "2012-10-17",
    Statement: [
      {
        Effect: "Allow",
        Action: [
          "glue:GetDatabases",
          "glue:GetTables",
          "glue:GetTable"
        ],
        Resource: "*"
      },
      {
        Effect: "Allow",
        Action: [
          "s3:GetObject"
        ],
        Resource: "arn:aws:s3:::*/*"
      }
    ]
  }}
/>

The `s3:GetObject` permission is needed because Glue's `LoadTable` reads Iceberg metadata files from S3.



## Example Configuration

```yaml

# REST catalog (default)
uri: "http://localhost:8181"
warehouse: "my-warehouse"
credential: "client-id:client-secret"
tags:
  - "iceberg"

# Glue catalog:
# catalog_type: "glue"
# credentials:
#   region: "us-east-1"
# glue_catalog_id: "123456789012"  # optional, defaults to caller's account

```

## Configuration
The following configuration options are available:

| Property | Type | Required | Description |
|----------|------|----------|-------------|
| catalog_type | string | false | Catalog backend type |
| credential | string | false | Credential for OAuth2 client credentials authentication |
| credentials | AWSCredentials | false | AWS credentials configuration |
| external_links | []ExternalLink | false | External links to show on all assets |
| filter | Filter | false | Filter discovered assets by name (regex) |
| glue_catalog_id | string | false | AWS Glue Data Catalog ID (defaults to caller's account) |
| include_namespaces | bool | false | Whether to discover namespaces as assets |
| include_tags | []string | false | List of AWS tags to include as metadata. By default, all tags are included. |
| include_views | bool | false | Whether to discover views |
| prefix | string | false | Optional prefix for the REST catalog |
| properties | map[string]string | false | Additional catalog properties |
| tags | TagsConfig | false | Tags to apply to discovered assets |
| tags_to_metadata | bool | false | Convert AWS tags to Marmot metadata |
| token | string | false | Bearer token for authentication |
| uri | string | false | REST catalog URI (required for catalog_type=rest) |
| warehouse | string | false | Warehouse identifier |

## Available Metadata

The following metadata fields are available:

| Field | Type | Description |
|-------|------|-------------|
| current_snapshot_id | string | Current snapshot ID |
| format_version | int | Iceberg format version (1, 2, or 3) |
| format_version | int | View format version |
| last_updated_ms | int64 | Last update timestamp in milliseconds |
| location | string | Table data location |
| location | string | Default location for tables |
| location | string | View metadata location |
| namespace | string | Namespace path |
| partition_spec | string | Partition specification |
| schema_field_count | int | Number of schema fields |
| schema_field_count | int | Number of schema fields |
| snapshot_count | int | Number of snapshots |
| sort_order | string | Sort order specification |
| sql | string | SQL definition of the view |
| sql_dialect | string | SQL dialect of the view definition |
| table_uuid | string | Table UUID |
| total_data_files | string | Total data file count |
| total_file_size | string | Total file size in bytes |
| total_records | string | Total record count |
| view_uuid | string | View UUID |