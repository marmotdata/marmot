---
title: Firebase
description: Discovers Firestore collections and Realtime Database instances from Firebase projects.
status: experimental
---

# Firebase

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


The Firebase plugin discovers the data stores in a Firebase project: Firestore databases with their collections and subcollections, and Realtime Database instances.

Firestore stores no schema, so each collection's fields are inferred from a sample of its documents. A field missing from any sampled document, or null in one, is marked nullable, and a field seen with two different types is recorded as `mixed`.

Cloud Storage for Firebase buckets are not discovered here. They are ordinary GCS buckets, and the [Google Cloud Storage](./Google%20Cloud%20Storage.md) plugin already catalogues them under the same identity.

## Subcollections

A Firestore subcollection lives under a document, so a `lines` subcollection under `orders` really exists once per order. Those copies share a shape, so the plugin catalogues them as one asset with the parent document id left out of the name: `firestore/(default)/orders/lines`. The fields are inferred from documents read across all the parent documents it sampled. The `collection_group_id` metadata field is the id a Firestore collection group query uses to reach every copy.

## Databases

Leaving `databases` empty lists every Firestore database in the project through the admin API. The Firestore emulator does not implement that call, so name the databases explicitly when pointing the plugin at one.

## Required Permissions

The service account needs read access to the project's databases:

- `datastore.databases.list` and `datastore.databases.get` to list Firestore databases
- `datastore.entities.list` to read the collections and sample their documents
- `firebasedatabase.instances.list` to list Realtime Database instances
- `firebase.projects.get` to read the project name and number

The predefined roles `roles/datastore.viewer` and `roles/firebase.viewer` cover all of these.

## Example Configuration

```yaml

project_id: "my-firebase-project"
credentials_file: "/path/to/service-account.json"
sample_documents: 50
max_collection_depth: 2
include_realtime_database: true
filter:
  include:
    - "^firestore/.*"
  exclude:
    - ".*_tmp$"
tags:
  - "firebase"
  - "firestore"

```

## Configuration
The following configuration options are available:

| Property | Type | Required | Description |
|----------|------|----------|-------------|
| credentials_file | string | false | Path to service account JSON file |
| credentials_json | string | false | Service account JSON content |
| databases | []string | false | Firestore database IDs to scan. Every database is listed from the API when this is empty. The Firestore emulator has no such API, so name the databases here when pointing at one |
| disable_auth | bool | false | Disable authentication, for local testing |
| endpoint | string | false | Custom endpoint URL, for testing against a local server |
| external_links | []ExternalLink | false | External links to show on all assets |
| filter | Filter | false | Filter discovered assets by name (regex) |
| include_project_details | bool | false | Whether to read the Firebase project name and number |
| include_realtime_database | bool | false | Whether to discover Realtime Database instances |
| include_subcollections | bool | false | Whether to descend into subcollections |
| max_collection_depth | int | false | How many levels of subcollection to descend |
| project_id | string | true | Google Cloud project ID |
| sample_documents | int | false | How many documents to read per collection to infer its fields |
| tags | TagsConfig | false | Tags to apply to discovered assets |

## Available Metadata

The following metadata fields are available:

| Field | Type | Description |
|-------|------|-------------|
| collection_group_id | string | ID a collection group query uses to reach every copy of a subcollection |
| collection_id | string | Collection ID, the last segment of the path |
| collection_path | string | Collection path with parent document IDs left out |
| column_name | string | Document field name |
| concurrency_mode | string | Default transaction concurrency control mode |
| create_time | string | When the database was created |
| data_type | string | Inferred Firestore type, or mixed when the sample disagreed |
| database_id | string | Firestore database ID, (default) for the default database |
| database_kind | string | Which Firebase database this is: firestore or realtime |
| database_type | string | FIRESTORE_NATIVE or DATASTORE_MODE |
| database_url | string | Hostname the instance is served on |
| delete_protection | string | Whether the database is protected from deletion |
| depth | int | Nesting level, 1 for a root collection |
| earliest_version_time | string | Oldest timestamp a read can ask for |
| firebase_project_display_name | string | Project display name |
| firebase_project_id | string | Firebase project ID |
| firebase_project_number | int64 | Google-assigned project number |
| free_tier | bool | Whether the database is eligible for the free tier |
| instance_id | string | Realtime Database instance ID |
| instance_type | string | DEFAULT_DATABASE or USER_DATABASE |
| is_nullable | bool | Whether the field was absent or null in any sampled document |
| location_id | string | Region the database runs in |
| parent_path | string | Path of the parent collection, for subcollections |
| point_in_time_recovery | string | Whether point in time recovery is enabled |
| sampled_documents | int | How many documents were read to infer the fields |
| state | string | Lifecycle state, for example ACTIVE or DISABLED |
| uid | string | System-generated database UUID |
| update_time | string | When the database resource was last changed |
| version_retention_period | string | How long past versions of the data are readable |
