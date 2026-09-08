---
title: Athena
description: This plugin discovers databases, tables, workgroups and saved queries from Amazon Athena.
status: experimental
---

# Athena

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


The Athena plugin discovers data catalogs, databases, tables, views, workgroups and saved queries from Amazon Athena.

## Asset Identity

Athena keeps no catalog of its own: the tables it queries live in the Glue Data Catalog. Databases, tables and views are therefore filed under the `Glue` provider with the same names the [Glue](./Glue.md) plugin uses, so an Athena run and a Glue run land on one asset instead of two half-populated ones. What Athena knows about such an asset is kept under the `athena` key of its metadata and in its own `Athena` entry under Asset Sources.

Workgroups, saved queries and data catalogs are Athena's own objects and are filed under the `Athena` provider. A saved query is named `<workgroup>/<name>`, because two workgroups can each hold a query of the same name.

## Metadata API

Databases and tables are read through the Athena metadata API, which also serves federated catalogs that Glue knows nothing about. When that API does not answer for a Glue-backed catalog, the plugin reads the Glue Data Catalog directly instead. Set `metadata_api` to `athena` or `glue` to pin the choice.

## Lineage

- A data catalog contains its databases, and a database contains its tables and views.
- A workgroup contains its saved queries, and every table a saved query reads feeds it.
- The S3 bucket behind a table's storage location feeds the table, and a workgroup produces the bucket its query results are written to. Bucket assets are not created here: those edges appear once the [S3](./S3.md) plugin has catalogued the bucket.

## Limitations

Query history is not read, so this plugin produces no usage statistics and no query-derived table-to-table lineage.

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
          "athena:ListDataCatalogs",
          "athena:GetDataCatalog",
          "athena:ListDatabases",
          "athena:ListTableMetadata",
          "athena:ListWorkGroups",
          "athena:GetWorkGroup",
          "athena:ListNamedQueries",
          "athena:BatchGetNamedQuery",
          "athena:GetNamedQuery",
          "athena:ListTagsForResource",
          "glue:GetDatabases",
          "glue:GetTables",
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
          "athena:ListDataCatalogs",
          "athena:ListDatabases",
          "athena:ListTableMetadata"
        ],
        Resource: "*"
      }
    ]
  }}
/>

`sts:GetCallerIdentity` is only needed with `tags_to_metadata`, because Athena does not return workgroup or catalog ARNs and the plugin builds them from the account id.

## AWS Configuration

See [AWS Configuration](./Shared%20Configuration/AWS%20Configuration.md) for the supported AWS configuration options.



## Example Configuration

```yaml

credentials:
  region: "us-east-1"
  profile: "production"
catalogs:
  - "AwsDataCatalog"
exclude_databases:
  - "default"
include_workgroups: true
include_saved_queries: true
tags:
  - "aws"
  - "athena"

```

## Configuration
The following configuration options are available:

| Property | Type | Required | Description |
|----------|------|----------|-------------|
| catalogs | []string | false | Data catalogs to discover. All catalogs when empty |
| credentials | AWSCredentials | false | AWS credentials configuration |
| databases | []string | false | Databases to discover. All databases when empty |
| discover_lineage | bool | false | Whether to discover lineage between catalogs, databases, tables, buckets and saved queries |
| exclude_catalogs | []string | false | Data catalogs to skip |
| exclude_databases | []string | false | Databases to skip |
| external_links | []ExternalLink | false | External links to show on all assets |
| filter | Filter | false | Filter discovered assets by name (regex) |
| include_columns | bool | false | Whether to include table columns |
| include_partitions | bool | false | Whether to include partition keys |
| include_saved_queries | bool | false | Whether to catalog saved queries |
| include_tags | []string | false | List of AWS tags to include as metadata. By default, all tags are included. |
| include_workgroups | bool | false | Whether to catalog workgroups |
| metadata_api | string | false | Which API reads databases and tables: auto, athena or glue |
| tags | TagsConfig | false | Tags to apply to discovered assets |
| tags_to_metadata | bool | false | Convert AWS tags to Marmot metadata |
| workgroups | []string | false | Workgroups to discover. All workgroups when empty |

## Available Metadata

Table and database metadata is nested under the `athena` key, because those assets are shared with the Glue plugin.

The following metadata fields are available:

| Field | Type | Description |
|-------|------|-------------|
| athena.catalog | string | Data catalog holding the database or table |
| athena.catalog_type | string | Data catalog type (GLUE, HIVE, LAMBDA, FEDERATED) |
| athena.classification | string | Data format of the table (parquet, csv, json, ...) |
| athena.comment | string | Table comment |
| athena.compression | string | Compression codec of the table data |
| athena.created | string | Date and time the table was created |
| athena.database | string | Database holding the table |
| athena.description | string | Description of the database |
| athena.input_format | string | Hadoop input format class |
| athena.last_access | string | Date and time the table was last accessed |
| athena.location | string | Storage location of the table data |
| athena.location_uri | string | Storage location of the database |
| athena.output_format | string | Hadoop output format class |
| athena.parameters | map[string]string | Database parameters |
| athena.partition_keys | string | Partition key columns |
| athena.partition_projection | bool | Whether partition projection is enabled |
| athena.serde | string | Serialization library |
| athena.table_type | string | Table type (EXTERNAL_TABLE, VIRTUAL_VIEW, ...) |
| bytes_scanned_cutoff | int64 | Per-query limit on bytes scanned |
| created | string | Date and time the workgroup was created |
| database | string | Database the saved query runs against by default |
| description | string | Description of the catalog, workgroup or saved query |
| encryption | string | Encryption option for query results (SSE_S3, SSE_KMS, CSE_KMS) |
| encryption_kms_key | string | KMS key used to encrypt query results |
| enforce_configuration | bool | Whether the workgroup settings override client settings |
| engine_version | string | Effective Athena engine version |
| named_query_id | string | Athena identifier of the saved query |
| output_location | string | S3 location query results are written to |
| parameters | map[string]string | Catalog parameters, such as the Lambda function backing a federated catalog |
| publish_metrics | bool | Whether query metrics are published to CloudWatch |
| region | string | AWS region of the workgroup |
| requester_pays | bool | Whether queries may read requester pays buckets |
| selected_engine_version | string | Engine version the workgroup requested |
| state | string | Workgroup state (ENABLED, DISABLED) |
| type | string | Catalog type (GLUE, HIVE, LAMBDA, FEDERATED) |
| url | string | Link to the workgroup in the Athena console |
| workgroup | string | Workgroup holding the saved query |

Table and view columns carry the Hive type verbatim, so a nested type such as `array<struct<sku:string>>` survives intact.

| Field | Type | Description |
|-------|------|-------------|
| column_name | string | Column name |
| data_type | string | Hive type of the column, recorded verbatim |
| description | string | Column comment |
| is_nullable | bool | Whether null values are allowed, always true in Athena |
| is_partition_key | bool | Whether the column is a partition key |
