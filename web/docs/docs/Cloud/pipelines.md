---
sidebar_position: 7
title: Pipelines
description: On Marmot Cloud a pipeline authenticates to your cloud as itself, or reads a secret from your vault at run time. No credential is stored.
---

import { CalloutCard, DocCard, DocCardGrid } from '@site/src/components/DocCard';
import { ThemedImg } from '@site/src/components/ThemedImg';

# Pipelines

[Pipelines](../Populating/Pipelines.md) are part of open source Marmot: a plugin pointed at a source, run by the server on a schedule. This page covers what changes on Marmot Cloud, which is how a pipeline authenticates.

A pipeline on Marmot Cloud never holds a credential. For a source in Google Cloud, AWS or Azure it authenticates as itself. For anything else it reads a password from your secret manager just before each run.

<ThemedImg
  lightSrc="/img/cloud-pipelines-light.png"
  darkSrc="/img/cloud-pipelines-dark.png"
  alt="The Runs page listing pipelines with their plugin, schedule, last run and next run"
/>

## How keyless pipelines work

Your instance is an OIDC issuer. Before each run it mints a short-lived token for the pipeline, and your cloud exchanges that token for access. The token's subject is the pipeline's name, so you grant access per pipeline rather than to Marmot as a whole.

The pipeline resource exposes `issuer` and `subject`. Reference them from your cloud-side grant, as the guides do, and a rename updates both sides at once. Where Terraform's ordering forbids that, as in an AWS trust policy, hold the name in a local.

Federation is built into the plugin SDK, so every plugin for these clouds supports it. Each plugin's page on the [plugin registry](https://plugins.marmotdata.io) shows where the field sits in its config and which read-only role to grant.

| Cloud | Plugins | Config |
| --- | --- | --- |
| Google Cloud | BigQuery, Bigtable, Cloud Run, Firebase, Cloud Storage, GKE, Google Drive, Pub/Sub, Vertex AI | A Workload Identity provider, and optionally a service account to impersonate |
| AWS | Athena, DynamoDB, EKS, Firehose, Glue, Glue pipelines, Iceberg, Kinesis, Lambda, S3, SageMaker, SNS, SQS | A role to assume and a region |
| Azure | Blob Storage | A tenant id and client id, with no account key |

## Start with the guide for your cloud

Each guide takes you from an empty configuration to a running pipeline that authenticates as itself, then adds a source that needs a password.

<DocCardGrid>
  <DocCard
    title="Google Cloud"
    description="Your first pipeline: BigQuery, step by step"
    docId="Cloud/Guides/google"
    icon="mdi:google-cloud"
  />
  <DocCard
    title="AWS"
    description="Your first pipeline: Glue, step by step"
    docId="Cloud/Guides/aws"
    icon="mdi:aws"
  />
  <DocCard
    title="Azure"
    description="Your first pipeline: Blob Storage, step by step"
    docId="Cloud/Guides/azure"
    icon="mdi:microsoft-azure"
  />
</DocCardGrid>

## Sources that need a password

A self-managed database, a Kafka cluster with SASL, anything on-premises. Keep the password in your secret manager and point the pipeline at it through a [secret store](secret-stores.md). Marmot reads the value before each run and never stores it. The pipeline's `secrets` map names a config key and the secret that fills it; leave that key out of the config.

## Roles

Pipeline roles bind at the organization. Someone who writes a pipeline against a secret store also needs the user role on that store.

| Role | Grants |
| --- | --- |
| `ingestion.viewer` | See pipelines and runs. |
| `ingestion.admin` | Create, edit and delete pipelines. |
| `secretStore.user` | Attach a store's secrets to pipelines, without reading them. |

```hcl
resource "marmot_organization_iam_member" "data_engineering_runs_pipelines" {
  role   = "ingestion.admin"
  member = "group:${marmot_team.data_engineering.id}"
}

resource "marmot_secret_store_iam_member" "data_engineering_uses_prod" {
  secret_store_id = marmot_secret_store_google.prod.id
  role            = "secretStore.user"
  member          = "group:${marmot_team.data_engineering.id}"
}
```

Schedules, runs and pausing work as on open source Marmot; see [pipelines](../Populating/Pipelines.md).

<CalloutCard
  title="Browse every plugin"
  description="Configuration options, required permissions and examples for every source Marmot can catalog."
  href="https://plugins.marmotdata.io"
  buttonText="Plugin registry"
  variant="external"
  icon="mdi:puzzle"
/>
