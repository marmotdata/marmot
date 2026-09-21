---
sidebar_position: 3
title: Instances
description: Regions, sizes, versions and the lifecycle of a Marmot Cloud instance.
---

import { CalloutCard } from '@site/src/components/DocCard';
import { TipBox } from '@site/src/components/Steps';

# Instances

An instance is one Marmot server with its own hostname, Postgres database, user accounts and catalog. Two instances share nothing.

Most accounts should run one. Add a second only for a catalog that must be genuinely separate, such as a sandbox allowed to hold wrong data. Separating teams or environments inside one catalog is what [data products](/docs/data-products), tags and [access control](access-control.md) are for, and a second instance means a second set of users and a second copy of every pipeline.

## What an instance is made of

| Property | Changeable later |
| --- | --- |
| Name | Yes. A label only. |
| Hostname | No |
| Region | No |
| Size | Ask support |
| Version | Ask support |

### Regions

| Region | Location | Status |
| --- | --- | --- |
| `eu-west-1` | Frankfurt | Available |
| `us-east-1` | Virginia | Coming soon |
| `ap-southeast-1` | Singapore | Coming soon |

Choose the region closest to the systems you catalog, not to the people reading the catalog. A pipeline spends its time on round trips to your database or warehouse; the UI is a handful of requests per page and is fine from anywhere. Data residency in an unlisted region is part of Enterprise, so [talk to us](mailto:support@marmotdata.io) before launching.

### Sizes

| Size | vCPU | Memory | Good for |
| --- | --- | --- | --- |
| Standard | 1 | 2 GB | Evaluation and small catalogs. Included in every plan. |
| Performance | 2 | 8 GB | Day-to-day use by a team. Add-on. |
| Max | 4 | 16 GB | Large catalogs, deep lineage, many agents at once. Add-on. |

Size sets CPU and memory. The asset limit is a [plan limit](plans.md) and is independent. Start on Standard and size up when search or the lineage graph feels slow specifically while the instance is busy.

<TipBox variant="info" title="Changing size or version">
Not yet self-service. [Email support](mailto:support@marmotdata.io) and we schedule it. Data, configuration and hostname are unaffected.
</TipBox>

## Lifecycle

**Provisioning** allocates compute, creates an isolated Postgres cluster with backups, rolls out the server and MCP endpoint, and issues DNS and TLS for your hostname. It runs on the platform, not in your browser, and takes about a minute.

**Stopping** scales the instance to zero. Everything is kept and starting brings the same instance back. A stopped instance still occupies its plan slot and is still billed; deleting is what stops the bill.

**Degraded** means the platform is struggling to keep the instance rolled out. It is billed normally, retried continuously and usually recovers. **Failed** means provisioning never completed; it is not billed and not counted, so delete it and launch again.

**Deleting** removes the catalog, the database and its backups, the encryption key, the users and the access policy, with no grace period and no undo. Export first with the [API](/docs/api-reference) or [CLI](/docs/cli).

## Your hostname

Every instance gets a generated hostname under `marmotdata.cloud`. On Scale and Enterprise you can use a domain you control instead, such as `catalog.acme.com`. The hostname is fixed at launch, so [tell us before you create the instance](mailto:support@marmotdata.io).

This matters beyond branding. The hostname is the instance's OIDC issuer, which ends up in trust policies in your cloud accounts for [keyless pipelines](pipelines.md#how-keyless-pipelines-work) and [secret stores](secret-stores.md). A hostname you own is one you keep if the instance ever moves.

## The first administrator

A new instance has one account, `admin`, with a generated password shown in the console. Marmot requires a new password at first sign-in, after which the generated one stops working.

<CalloutCard
  title="Connect your identity provider first"
  description="Configure SSO while the instance still has one user, and every account that ever exists on it comes from your IdP."
  docId="Cloud/single-sign-on"
  buttonText="Set up SSO"
  icon="mdi:login-variant"
/>

## The MCP endpoint

Every instance serves the [Marmot MCP server](/docs/MCP) at `https://<your-host>/mcp`, with the same authentication as the rest of the API. Every client integration in these docs works once pointed at your host.

Give each agent its own [service account](access-control.md#service-accounts) rather than a person's credentials. It can be scoped to the assets it needs, its activity is attributable, and revoking it locks nobody out.
