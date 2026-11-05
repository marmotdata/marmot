---
sidebar_position: 3
title: Instances
description: How Marmot Cloud instances relate to the console, how compute units and billing work and what happens when an instance stops or is deleted.
---

import { TipBox } from '@site/src/components/Steps';
import { ThemedImg } from '@site/src/components/ThemedImg';

# Instances

An instance is one Marmot server with its own hostname, database, user accounts and catalog, and two instances share nothing. Instances run in Europe and are upgraded automatically to the latest Marmot release; data residency elsewhere and VPC peering are part of an Enterprise contract.


## The console and your instance

The console at `cloud.marmotdata.io` manages your account: plan, compute units, members and instances. Everything in the catalog, including pipelines, service accounts and grants, lives in the instance itself.

An instance's page in the console, with its address, CLI login command and MCP endpoint under **Connect**:

<ThemedImg
  lightSrc="/img/cloud-console-instance-light.png"
  darkSrc="/img/cloud-console-instance-dark.png"
  alt="The console page for the Kelp catalog instance, showing its region and its Address, CLI and MCP endpoint under Connect"
  size="panel"
/>

Your instance serves three addresses:

| Address | What it is |
| --- | --- |
| `https://<instance>.marmotdata.cloud` | The UI |
| `https://<instance>.marmotdata.cloud/api/v1` | The [REST API](/docs/api-reference) |
| `https://<instance>.marmotdata.cloud/api/v1/mcp` | The [MCP endpoint](/docs/MCP), with the same authentication as the API |

Give each agent its own [service account](service-accounts.md) rather than a person's credentials.

## How many instances

Free, Team and Scale include 1 instance and Enterprise includes 3.

Use a second instance only for a catalog that must be genuinely separate, such as a sandbox that's allowed to hold wrong data. It means a second set of users and a second copy of every pipeline. To separate teams inside one catalog, use [data products](/docs/data-products), tags and [access control](access-control.md).

## Compute units

Capacity is measured in compute units. Each unit gives the instance dedicated compute and database capacity and adds 5,000 assets to its limit. The limit is a hard cap: once the catalog reaches it, Marmot refuses new assets until you add units.

| Plan | Compute | Asset limit |
| --- | --- | --- |
| Free | Shared | 500 |
| Team | 1 to 3 units | 5,000 per unit |
| Scale | 4 or more units | 5,000 per unit |
| Enterprise | Set in your contract | Set in your contract |

An account owner changes units under **Billing** with the **Compute units** slider, then selects **Update compute units**. A running instance picks up the change automatically. Units are billed monthly and prorated.

## Plans and billing

You pay for your plan and compute units, not for each instance. Stopping or deleting an instance doesn't change your bill; to stop paying, an owner switches the account to Free under **Billing**.

An instance keeps the plan it was launched on. Upgrading moves your instances up with it, so a Free instance becomes a Team instance without a relaunch. Moving down is refused while an instance was launched on a higher plan, so delete it first or [contact us](mailto:support@marmotdata.io).

## Hostnames

An instance's `marmotdata.cloud` hostname never changes, because it's also the OIDC issuer your cloud accounts trust for [keyless pipelines](pipelines.md#how-keyless-pipelines-work) and [secret stores](secret-stores.md). It stays the issuer after you [add a custom domain](custom-domains.md).

## Lifecycle

A stopped instance keeps its data, but its UI, API and MCP endpoint are unreachable until you select **Start instance**. A **Degraded** instance is having trouble staying up and retries automatically; [contact support](mailto:support@marmotdata.io) if it doesn't recover. A **Failed** instance never finished launching; delete it and launch again.

A Free instance sleeps after 30 minutes without a request and wakes on the next one, which takes a minute or two. Scheduled ingestion doesn't run while it sleeps, so a Free catalog refreshes when someone or something uses it.

Marmot Cloud stops or deletes instances for you:

- On any plan, when the plan no longer covers an instance, it's stopped. **Start instance** is then refused until you upgrade or delete the extra instances.
- On Free, an instance that hasn't been used for 90 days is deleted with its data. Use means a request to the instance, from the UI, the API or MCP, or anyone in the account signing in to the console. The account's owners are emailed 30 days and 7 days before, and any use before the date in the email cancels the deletion.

### Deleting an instance

Deleting removes the catalog, its users, its encryption key and its custom domain.

<TipBox variant="warning" title="Deleting an instance can't be undone">
There's no grace period and no restore from the console. Export what you need first with the [API](/docs/api-reference) or the [CLI](/docs/cli).
</TipBox>

## Next steps

- [Use your own domain](custom-domains.md): serve the instance on a hostname you control.
- [Access control](access-control.md): decide who can see and change what.
- [Security and isolation](security.md): how instances are kept apart.
