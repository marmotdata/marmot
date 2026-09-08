---
title: NiFi
description: Discovers process groups, processors and data flow lineage from Apache NiFi.
status: experimental
---

# NiFi

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


The NiFi plugin discovers process groups and processors from Apache NiFi through its REST API. Each process group becomes a Pipeline named by its path from the root group (`NiFi Flow/Ingest`), each processor a Task named `<group path>/<processor name>`. Connections become lineage: a Pipeline CONTAINS its Tasks and child Pipelines, a Task DEPENDS_ON the Task a connection leads to, and a Pipeline DEPENDS_ON the Pipeline an output port feeds through an input port.

Well-known processors are also linked to the data they touch. PutS3Object, FetchS3Object, ListS3, PutGCSObject, FetchGCSObject, ListGCSBucket, PutAzureBlobStorage and FetchAzureBlobStorage link to the bucket or container they name. PublishKafka and ConsumeKafka link to Kafka topics, and the topics are created as Kafka assets so the edges have somewhere to land. PutDatabaseRecord, QueryDatabaseTable and GenerateTableFetch link to the table they name, using the JDBC URL of their connection pool to work out which database plugin owns it (PostgreSQL, MySQL, MariaDB, SQL Server, Oracle). PutElasticsearchRecord and PutElasticsearchJson link to the index. Property values that use expression language or parameters are skipped, since they are only known at run time.

Sensitive processor properties are masked. Ports are only discovered when `include_ports` is on.

## Prerequisites

- NiFi 1.x or 2.x with the REST API reachable from where Marmot runs
- A user with `view the user interface` and read access to the process groups, plus read access to controller services for table lineage
- `verify_ssl: false` or `ca_cert` when NiFi runs with its self-signed certificate

:::tip[Authentication]
Single-user and LDAP installs take `username` and `password`. An existing bearer token goes in `token`. Installs secured with client certificates take `client_cert` and `client_key`. Leave all three empty for an unsecured HTTP install.
:::

:::note[Run history]
NiFi records provenance events rather than runs, so the plugin emits no run history.
:::

## Example Configuration

```yaml

host: "https://nifi.example.com:8443"
username: "marmot"
password: "${NIFI_PASSWORD}"
verify_ssl: false
include_processors: true
include_ports: false
discover_lineage: true
tags:
  - "nifi"

```

## Configuration
The following configuration options are available:

| Property | Type | Required | Description |
|----------|------|----------|-------------|
| ca_cert | string | false | Path to a PEM CA certificate to trust |
| client_cert | string | false | Path to a PEM client certificate for mutual TLS (used instead of a login) |
| client_key | string | false | Path to the PEM private key of the client certificate |
| discover_lineage | bool | false | Link well-known processors to the buckets, topics, tables and indexes they read or write |
| external_links | []ExternalLink | false | External links to show on all assets |
| filter | Filter | false | Filter discovered assets by name (regex) |
| host | string | true | NiFi base URL (e.g. https://nifi.example.com:8443) |
| include_ports | bool | false | Discover input and output ports as Task assets |
| include_processors | bool | false | Discover processors as Task assets |
| password | string | false | Password for the username |
| root_process_group | string | false | Id of the process group to start from (defaults to the root group) |
| tags | TagsConfig | false | Tags to apply to discovered assets |
| token | string | false | Bearer token to use instead of a username and password |
| username | string | false | Username for single-user or LDAP login. Leave all credentials empty for an unsecured HTTP install |
| verify_ssl | bool | false | Verify the NiFi TLS certificate. Set to false for the self-signed certificate NiFi ships with |

## Available Metadata

The following metadata fields are available:

| Field | Type | Description |
|-------|------|-------------|
| comments | string | Comments set on the process group or processor |
| connection_count | int | Connections directly in the group |
| consumers | []string | Tasks that consume from the topic |
| disabled_count | int | Components disabled in the group and its children |
| group_id | string | Id of the process group holding the processor |
| id | string | Process group or processor id |
| input_port_count | int | Input ports of the group |
| invalid_count | int | Components that cannot start because their configuration is invalid |
| nifi_version | string | NiFi release the instance runs |
| output_port_count | int | Output ports of the group |
| parameter_context | string | Name of the bound parameter context |
| parent_id | string | Id of the parent process group |
| path | string | Group names from the root, joined with / |
| pipeline | string | Path of the process group holding the processor |
| port_type | string | INPUT_PORT or OUTPUT_PORT |
| processor_count | int | Processors directly in the group |
| producers | []string | Tasks that publish to the topic |
| properties | map[string]string | Configured properties, with sensitive values masked |
| relationships | []string | Names of the processor's relationships |
| running_count | int | Components running in the group and its children |
| scheduling_period | string | How often the processor is scheduled |
| scheduling_strategy | string | TIMER_DRIVEN or CRON_DRIVEN |
| state | string | RUNNING, STOPPED, DISABLED or INVALID |
| stopped_count | int | Components stopped in the group and its children |
| topic_name | string | Kafka topic name |
| type | string | Processor class name (e.g. PutS3Object) |
| type_full | string | Fully qualified processor class name |
| url | string | Link to the group or processor in the NiFi UI |
