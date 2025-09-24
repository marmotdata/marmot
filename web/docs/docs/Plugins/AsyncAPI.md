---
title: AsyncAPI
description: This plugin enables fetching data from AsyncAPI specifications.
status: experimental
---

# AsyncAPI

**Status:** experimental

## Example Configuration

```yaml

spec_path: "/app/api-specs"
resolve_external_docs: true
tags:
  - "asyncapi"
  - "specifications"

```

## Configuration
The following configuration options are available:

| Property | Type | Required | Description |
|----------|------|----------|-------------|
| aws | AWSConfig | false |  |
| external_links | []ExternalLink | false |  |
| global_documentation | []string | false |  |
| global_documentation_position | string | false |  |
| metadata | MetadataConfig | false |  |
| resolve_external_docs | bool | false |  |
| spec_path | string | false |  |
| tags | TagsConfig | false |  |

## Available Metadata

The following metadata fields are available:

| Field | Type | Description |
|-------|------|-------------|
| binding_is | string | AMQP binding type (queue or routingKey) |
| cleanup_policy | []string | Topic cleanup policies |
| cluster_id | string | Kafka cluster ID |
| content_deduplication | bool | Whether content-based deduplication is enabled |
| deduplication_scope | string | Scope of deduplication if enabled |
| delete_retention_ms | int64 | Time to retain deleted messages |
| delivery_delay | int | Delivery delay in seconds |
| description | string | Description of the resource |
| dlq_name | string | Name of the Dead Letter Queue |
| environment | string | Environment the resource belongs to |
| exchange_auto_delete | bool | Exchange auto delete flag |
| exchange_durable | bool | Exchange durability flag |
| exchange_name | string | Exchange name |
| exchange_type | string | Exchange type (topic, fanout, direct, etc.) |
| exchange_vhost | string | Exchange virtual host |
| fifo_queue | bool | Whether this is a FIFO queue |
| fifo_throughput_limit | string | FIFO throughput limit type |
| max_message_bytes | int | Maximum message size |
| max_receive_count | int | Maximum receives before sending to DLQ |
| message_retention_period | int | Message retention period in seconds |
| name | string | Name of the SQS queue |
| ordering_type | string | SNS topic ordering type |
| partitions | int | Number of partitions |
| queue_auto_delete | bool | Queue auto delete flag |
| queue_durable | bool | Queue durability flag |
| queue_exclusive | bool | Queue exclusivity flag |
| queue_name | string | Queue name |
| queue_vhost | string | Queue virtual host |
| receive_message_wait_time | int | Long polling wait time in seconds |
| replicas | int | Number of replicas |
| retention_bytes | int64 | Maximum size of the topic |
| retention_ms | int64 | Message retention period in milliseconds |
| service_name | string | Name of the service that owns the resource |
| service_version | string | Version of the service |
| topic_arn | string | SNS Topic Name/ARN |
| visibility_timeout | int | Visibility timeout in seconds |