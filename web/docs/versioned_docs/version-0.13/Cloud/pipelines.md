---
sidebar_position: 6
title: Pipelines
description: On Marmot Cloud a pipeline authenticates to your cloud as itself, or reads a secret from your vault at run time. No credential is stored.
---

import { ThemedImg } from '@site/src/components/ThemedImg';

# Pipelines

A pipeline on Marmot Cloud doesn't need to hold a credential. For a source in Google Cloud, AWS or Azure it authenticates as itself, and for anything else it reads a password from your secret manager shortly before each run.


[Pipelines](../Populating/Pipelines.md) work as they do in open source Marmot: a plugin pointed at a source, run on a schedule. Your pipelines are listed under **Plugins** in your instance:

<ThemedImg
  lightSrc="/img/cloud-pipelines-light.png"
  darkSrc="/img/cloud-pipelines-dark.png"
  alt="The Plugins page in a Marmot Cloud instance, listing pipelines with their schedule, recent runs and the assets each one catalogs"
/>

## How keyless pipelines work

Your instance is an OIDC issuer, and your cloud exchanges the tokens it signs for access:

<figure className="doc-diagram" role="img" aria-label="Before a run, your Marmot instance signs a token for the pipeline. Your cloud checks it against the instance's published keys and returns short-lived credentials, which the pipeline uses to read the source.">
  <svg viewBox="0 0 640 120" xmlns="http://www.w3.org/2000/svg">
    <rect className="doc-diagram__box doc-diagram__box--root" x="8" y="30" width="160" height="56" rx="6" />
    <text className="doc-diagram__label" x="88" y="54" textAnchor="middle">Your instance</text>
    <text className="doc-diagram__label doc-diagram__label--child" x="88" y="74" textAnchor="middle">signs a token</text>
    <path className="doc-diagram__line" d="M168 58 H232" />
    <path className="doc-diagram__line" d="M224 52 L232 58 L224 64" />
    <rect className="doc-diagram__box" x="240" y="30" width="160" height="56" rx="6" />
    <text className="doc-diagram__label" x="320" y="54" textAnchor="middle">Your cloud</text>
    <text className="doc-diagram__label doc-diagram__label--child" x="320" y="74" textAnchor="middle">swaps it for credentials</text>
    <path className="doc-diagram__line" d="M400 58 H464" />
    <path className="doc-diagram__line" d="M456 52 L464 58 L456 64" />
    <rect className="doc-diagram__box" x="472" y="30" width="160" height="56" rx="6" />
    <text className="doc-diagram__label" x="552" y="54" textAnchor="middle">The source</text>
    <text className="doc-diagram__label doc-diagram__label--child" x="552" y="74" textAnchor="middle">read by the pipeline</text>
  </svg>
</figure>

The instance publishes its signing key at `/.well-known/openid-configuration` and `/.well-known/jwks.json`. Each token describes one pipeline:

- **Issuer**: `https://<instance>.marmotdata.cloud`. It stays the same after you add a custom domain.
- **Subject**: `pipeline:<pipeline-name>`, so you grant access to each pipeline rather than to Marmot as a whole.
- **Audience**: set by the plugin, as the table below shows.
- **Lifetime**: 5 minutes. Marmot issues a fresh one every 2 minutes for as long as it needs access.

A pipeline federates as soon as its config names the cloud identity to use:

| Cloud | Plugins | Config that federates | Audience |
| --- | --- | --- | --- |
| Google Cloud | BigQuery, Bigtable, Cloud Run, Firebase, Cloud Storage, GKE, Google Drive, Pub/Sub, Vertex AI | A Workload Identity provider, and optionally a service account to impersonate | `https://iam.googleapis.com/<provider>` |
| AWS | Athena, DynamoDB, EKS, Firehose, Glue, Glue pipelines, Iceberg, Kinesis, Lambda, S3, SageMaker, SNS, SQS | A role ARN and a region | `sts.amazonaws.com` |
| Azure | Blob Storage | A tenant ID and client ID, with no account key | `api://AzureADTokenExchange` |

The guide for your cloud sets one up end to end: [Google Cloud](Guides/google.md), [AWS](Guides/aws.md) or [Azure](Guides/azure.md).

## Pipeline names are identities

The subject follows the pipeline's name, so renaming a pipeline breaks its grants in your cloud until you update them. A new pipeline given a deleted pipeline's name gets that pipeline's access, so remove the grants on your cloud's side when you delete a pipeline.

If you manage pipelines with Terraform, the `marmot_pipeline` resource exposes `issuer` and `subject`. Reference them from the grant on your cloud's side, and a rename updates both sides at once.

## Sources that need a password

A self-managed database, a Kafka cluster with SASL or anything else that can't federate needs a password. Keep it in your secret manager and [use it through a secret store](secret-stores.md#use-a-secret-in-a-pipeline). Marmot reads the value before each run and never stores it.

## Sources on a private network

Your instance connects to public addresses only. For a source on a private network, run the plugin inside that network with `marmot plugin`, which pushes its results out to your instance. [Serving queries from your network](../Configure/serving-plugins.md) describes it.

## Roles

Pipeline roles are granted across the whole catalog:

- `ingestion.viewer` sees pipelines and runs.
- `ingestion.admin` creates, edits and deletes pipelines.
- `secretStore.user`, granted on a store, attaches that store's secrets to pipelines without reading them.

A team that writes pipelines usually needs `ingestion.admin` across the catalog and `secretStore.user` on each store it uses. [Grant a role](grant-access.md#grant-a-role) shows how.

## Next steps

- [Connect your cloud](Guides/google.md): your first keyless pipeline on Google Cloud, AWS or Azure.
- [Secret stores](secret-stores.md): passwords for everything else.
- [Plugin directory](https://plugins.marmotdata.io): configuration for every source Marmot can catalog.
