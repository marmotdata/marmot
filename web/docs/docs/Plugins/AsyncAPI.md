---
title: AsyncAPI
description: This plugin ingests metadata from AsyncAPI v3 specifications, discovering services, channels, and message schemas.
status: experimental
---

# AsyncAPI

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



## Example Configuration

```yaml

spec_path: "/app/asyncapi-specs"
environment: "production"
discover_services: true
discover_channels: true
discover_messages: true
tags:
  - "asyncapi"
  - "event-driven"
channel_filter:
  include:
    - "orders.*"
    - "users.*"

```

## Configuration
The following configuration options are available:

| Property | Type | Required | Description |
|----------|------|----------|-------------|
| channel_filter | plugin.Filter | false | Filter channels by name pattern |
| discover_channels | bool | false | Create channel/topic assets from channels and bindings |
| discover_messages | bool | false | Attach message schemas to channel assets |
| discover_services | bool | false | Create service assets from AsyncAPI info |
| environment | string | false | Environment name (e.g., production, staging) |
| external_links | []ExternalLink | false | External links to show on all assets |
| spec_path | string | false | Path to AsyncAPI spec file or directory containing specs |
| tags | TagsConfig | false | Tags to apply to discovered assets |

## Available Metadata

The following metadata fields are available:

| Field | Type | Description |
|-------|------|-------------|
| allowed_regions | []string | Allowed persistence regions |
| asyncapi_version | string | AsyncAPI specification version |
| binding_is | string | AMQP binding type (queue or routingKey) |
| channel_address | string | Address/topic of the channel |
| channel_count | int | Number of channels |
| channel_name | string | Name of the channel in the AsyncAPI spec |
| cleanup_policy | []string | Topic cleanup policies |
| contact_email | string | Contact email address |
| contact_name | string | Contact person name |
| contact_url | string | Contact URL |
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
| license | string | License name |
| license_url | string | License URL |
| max_message_bytes | int | Maximum message size |
| max_receive_count | int | Maximum receives before sending to DLQ |
| message_retention_duration | string | Message retention duration |
| message_retention_period | int | Message retention period in seconds |
| operation_count | int | Number of operations |
| ordering_type | string | SNS topic ordering type |
| partitions | int | Number of partitions |
| protocols | []string | List of protocols used |
| queue_auto_delete | bool | Queue auto delete flag |
| queue_durable | bool | Queue durability flag |
| queue_exclusive | bool | Queue exclusivity flag |
| queue_name | string | Name of the SQS queue |
| queue_name | string | Queue name |
| queue_vhost | string | Queue virtual host |
| receive_message_wait_time | int | Long polling wait time in seconds |
| replicas | int | Number of replicas |
| retention_bytes | int64 | Maximum size of the topic |
| retention_ms | int64 | Message retention period in milliseconds |
| schema_encoding | string | Schema encoding format |
| schema_name | string | Schema name |
| servers | []string | List of server names |
| service_name | string | Name of the service that owns the resource |
| service_version | string | Version of the service |
| topic_arn | string | SNS Topic ARN |
| topic_name | string | Kafka topic name |
| topic_name | string | Google Pub/Sub topic name |
| topic_name | string | SNS Topic Name |
| visibility_timeout | int | Visibility timeout in seconds |