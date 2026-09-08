---
title: Firehose
description: Discovers Amazon Data Firehose delivery streams and their lineage from AWS accounts.
status: experimental
---

# Firehose

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
  docId="Populating/UI"
  buttonText="View Guide"
  variant="secondary"
  icon="mdi:cursor-default-click"
/>


The Firehose plugin discovers Amazon Data Firehose delivery streams across your AWS accounts. Each delivery stream becomes a `DeliveryStream` asset named after the stream, carrying its status, type, encryption, source and destination settings.

## Lineage

A delivery stream sits between a producer and a destination, so the plugin links it to both:

- The Kinesis stream or Kafka topic feeding it, as a `FEEDS` edge into the delivery stream.
- Each system it writes to, as a `PRODUCES` edge out of the delivery stream: S3 buckets, Redshift, Elasticsearch, OpenSearch, Snowflake and Iceberg tables, plus the Glue table an extended S3 destination converts its records against.

Splunk and HTTP endpoint destinations get no edge because Marmot has no asset for them; their endpoint is recorded in metadata instead.

The plugin creates none of those assets, it only points at them. Run the plugin that owns each system alongside this one, otherwise the edge has nothing to attach to and is dropped.

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
          "firehose:ListDeliveryStreams",
          "firehose:DescribeDeliveryStream",
          "firehose:ListTagsForDeliveryStream"
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
        Action: ["firehose:ListDeliveryStreams", "firehose:DescribeDeliveryStream"],
        Resource: "*"
      }
    ]
  }}
/>

## AWS Configuration

See [AWS Configuration](./Shared%20Configuration/AWS%20Configuration.md) for the supported AWS configuration options.



## Example Configuration

```yaml

credentials:
  region: "us-east-1"
  profile: "production"
discover_lineage: true
include_destination_config: true
tags_to_metadata: true
tags:
  - "aws"
  - "firehose"

```

## Configuration
The following configuration options are available:

| Property | Type | Required | Description |
|----------|------|----------|-------------|
| credentials | AWSCredentials | false | AWS credentials configuration |
| discover_lineage | bool | false | Link each stream to the systems it reads from and writes to |
| external_links | []ExternalLink | false | External links to show on all assets |
| filter | Filter | false | Filter discovered assets by name (regex) |
| include_destination_config | bool | false | Record the destination settings in metadata |
| include_tags | []string | false | List of AWS tags to include as metadata. By default, all tags are included. |
| tags | TagsConfig | false | Tags to apply to discovered assets |
| tags_to_metadata | bool | false | Convert AWS tags to Marmot metadata |

## Available Metadata

The following metadata fields are available:

| Field | Type | Description |
|-------|------|-------------|
| arn | string | ARN of the delivery stream |
| created_at | string | When the delivery stream was created |
| destination | map[string]any | Destination settings, with any value that could carry a credential redacted |
| destination_count | int | Number of destinations configured on the delivery stream |
| destination_type | string | Where records are written (s3, extended_s3, redshift, elasticsearch, opensearch, opensearch_serverless, splunk, http_endpoint, snowflake, iceberg) |
| encryption_key_type | string | Server-side encryption key type (AWS_OWNED_CMK, CUSTOMER_MANAGED_CMK) |
| encryption_status | string | Server-side encryption status |
| last_updated_at | string | When the delivery stream was last updated |
| region | string | AWS region the delivery stream lives in |
| source_kinesis_stream | string | Name of the Kinesis stream feeding the delivery stream |
| source_msk_cluster | string | Name of the MSK cluster feeding the delivery stream |
| source_msk_topic | string | Name of the MSK topic feeding the delivery stream |
| source_type | string | Where records come from (direct_put, kinesis, msk, database) |
| status | string | Delivery stream status (ACTIVE, CREATING, DELETING) |
| stream_type | string | Delivery stream type (DirectPut, KinesisStreamAsSource, MSKAsSource, DatabaseAsSource) |
| tags | map[string]string | AWS resource tags |
| version_id | string | Version of the delivery stream configuration |

### Destination Settings

The `destination` field is a sub-map. Which of these fields are present depends on the destination type.

| Field | Type | Description |
|-------|------|-------------|
| account_url | string | Snowflake account URL |
| bucket | string | S3 bucket name |
| buffering_interval_seconds | int | Buffer interval in seconds before delivery |
| buffering_size_mb | int | Buffer size in MB before delivery |
| catalog_arn | string | Iceberg catalog ARN |
| cluster_endpoint | string | Redshift cluster host, or Elasticsearch and OpenSearch cluster endpoint |
| collection_endpoint | string | OpenSearch Serverless collection endpoint |
| compression_format | string | S3 compression format |
| copy_columns | string | Redshift columns the copy command targets |
| copy_options | string | Redshift copy command options |
| database | string | Redshift or Snowflake database |
| domain_arn | string | Elasticsearch or OpenSearch domain ARN |
| error_output_prefix | string | S3 key prefix failed records are written under |
| file_extension | string | File extension of the delivered S3 objects |
| format_conversion_enabled | bool | Whether records are converted to a columnar format |
| glue_catalog_id | string | Glue catalog the conversion schema is read from |
| glue_database | string | Glue database the conversion schema is read from |
| glue_region | string | Region of the Glue catalog |
| glue_table | string | Glue table the conversion schema is read from |
| hec_endpoint | string | Splunk HTTP event collector endpoint |
| hec_endpoint_type | string | Splunk HTTP event collector endpoint type |
| index_name | string | Elasticsearch or OpenSearch index name |
| index_rotation_period | string | How often the index name is rotated |
| name | string | HTTP endpoint name |
| prefix | string | S3 key prefix records are written under |
| s3_backup_mode | string | Whether records are also backed up to S3 |
| schema | string | Snowflake schema |
| table | string | Redshift or Snowflake table |
| tables | []string | Iceberg destination tables |
| type_name | string | Elasticsearch or OpenSearch type name |
| url | string | HTTP endpoint URL |
| user | string | Snowflake user |
| username | string | Redshift user the copy command runs as |
| warehouse_location | string | Iceberg warehouse location |
