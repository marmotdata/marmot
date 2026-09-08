---
title: Google Pub/Sub
description: Discovers topics and subscriptions from Google Cloud Pub/Sub.
status: experimental
---

# Google Pub/Sub

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


The Google Pub/Sub plugin discovers topics and subscriptions from a Google Cloud project. Topics carry their labels, message storage policy, retention and schema; a topic with an Avro schema also gets a column list built from the schema's top-level fields. Subscriptions carry their delivery type, acknowledgement settings, filter and dead letter policy.

Lineage runs from a topic to each of its subscriptions, from a subscription to its dead letter topic, and from a subscription to the system it exports to: the BigQuery table or the Cloud Storage bucket. Those destinations are owned by the BigQuery and Google Cloud Storage plugins, so only the edge is created. A topic that ingests from Amazon Kinesis is linked from the Kinesis stream.

## Authentication

With neither `credentials_file` nor `credentials_json` set, Application Default Credentials are used, which covers Workload Identity, a Compute Engine service account and `GOOGLE_APPLICATION_CREDENTIALS`. The service account needs `roles/pubsub.viewer`. Without permission to read schemas, discovery logs a warning and continues without them.

## Emulator

Set `emulator_host` to a `host:port` address to run against the Pub/Sub emulator. The connection then uses plaintext gRPC with no credentials, and assets get no Google Cloud console links.

## Sample Messages

With `include_sample_messages: true`, asset previews read up to 20 messages for 5 seconds. A subscription is read directly; a topic borrows one of its pull subscriptions. Every message is nacked rather than acknowledged, so Pub/Sub redelivers it to the real consumer straight after.

## Example Configuration

```yaml

project_id: "company-streaming"
credentials_file: "/etc/marmot/pubsub-service-account.json"
include_subscriptions: true
include_schemas: true
include_dead_letter_topics: true
filter:
  include:
    - "^orders.*"
  exclude:
    - ".*-dlq$"
tags:
  - "pubsub"
  - "streaming"

```

## Configuration
The following configuration options are available:

| Property | Type | Required | Description |
|----------|------|----------|-------------|
| credentials_file | string | false | Path to a service account JSON file |
| credentials_json | string | false | Service account JSON content |
| emulator_host | string | false | Address of a Pub/Sub emulator, for example localhost:8085 |
| external_links | []ExternalLink | false | External links to show on all assets |
| filter | Filter | false | Filter discovered assets by name (regex) |
| include_dead_letter_topics | bool | false | Whether to record dead letter topics on subscriptions |
| include_sample_messages | bool | false | Whether to allow reading sample messages for asset previews |
| include_schemas | bool | false | Whether to attach topic schemas and their fields |
| include_subscriptions | bool | false | Whether to discover subscriptions |
| project_id | string | true | Google Cloud project ID |
| tags | TagsConfig | false | Tags to apply to discovered assets |

## Available Metadata

The following metadata fields are available:

| Field | Type | Description |
|-------|------|-------------|
| ack_deadline_seconds | int | Seconds a subscriber has to acknowledge a message |
| allowed_persistence_regions | []string | Regions the message storage policy allows |
| bigquery_state | string | Whether the BigQuery export is working (ACTIVE when it is) |
| bigquery_table | string | BigQuery table messages are written to |
| bigquery_use_topic_schema | bool | Whether the topic's schema is used to write the BigQuery rows |
| cloud_storage_bucket | string | Cloud Storage bucket messages are written to |
| cloud_storage_filename_prefix | string | Prefix of the objects written to the bucket |
| cloud_storage_filename_suffix | string | Suffix of the objects written to the bucket |
| cloud_storage_state | string | Whether the Cloud Storage export is working (ACTIVE when it is) |
| column_name | string | Avro field name |
| data_type | string | Avro field type, unions joined with a pipe |
| dead_letter_topic | string | Id of the topic undeliverable messages are forwarded to |
| delivery_type | string | How messages are delivered (pull, push, bigquery, cloud_storage) |
| description | string | The Avro field's doc string |
| detached | bool | Whether the subscription is detached from its topic and no longer receives messages |
| enable_message_ordering | bool | Whether messages with the same ordering key are delivered in order |
| exactly_once_delivery | bool | Whether exactly once delivery is enabled |
| expiration_ttl | string | How long the subscription can be inactive before it is deleted |
| filter | string | Expression selecting which messages are delivered |
| ingestion_source | string | External system Pub/Sub imports messages from (aws_kinesis, cloud_storage, azure_event_hubs, amazon_msk, confluent_cloud) |
| is_nullable | bool | Whether the field's union includes null |
| kms_key_name | string | Cloud KMS key protecting published messages |
| labels | map[string]string | Labels set on the topic or subscription |
| max_delivery_attempts | int | Deliveries attempted before a message goes to the dead letter topic |
| message_retention | string | How long unacknowledged messages are kept |
| project_id | string | Google Cloud project the resource belongs to |
| push_endpoint | string | URL messages are pushed to |
| retain_acked_messages | bool | Whether acknowledged messages are kept for replay |
| retention | string | How long published messages stay available to subscribers |
| schema | string | Id of the schema published messages are validated against |
| schema_encoding | string | Message encoding the schema is applied to (JSON, BINARY) |
| schema_revision | string | Revision id of the schema |
| schema_type | string | Schema type (AVRO, PROTOCOL_BUFFER) |
| state | string | Topic or subscription state |
| subscription_count | int | Number of subscriptions attached to the topic |
| subscription_name | string | Full resource name, projects/{project}/subscriptions/{subscription} |
| subscriptions | []string | Ids of the subscriptions attached to the topic |
| topic | string | Id of the topic the subscription reads |
| topic_name | string | Full resource name, projects/{project}/topics/{topic} |
| url | string | Link to the resource in the Google Cloud console |
