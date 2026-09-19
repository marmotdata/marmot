---
sidebar_position: 9
title: Plans and billing
description: What each Marmot Cloud plan includes, how instance sizes are billed on top, and how to change plan.
---

import { CalloutCard } from '@site/src/components/DocCard';
import { TipBox } from '@site/src/components/Steps';

# Plans and billing

The plan sets what an account may run and the limits on it. The size of each instance is billed on top as a separate line. Both live under Billing in the console, with current prices there and on the [pricing page](/pricing).

## What each plan includes

| | Free | Team | Scale | Enterprise |
| --- | --- | --- | --- | --- |
| Instances | 1 | 1 | 1 | 3 |
| Assets | 500 | 5,000 | 10,000 | Unlimited |
| Lookups per second | 2 | 10 | 50 | Unlimited |
| Sizes | Standard | All | All | All |
| Plugins | All | All | All | All |
| Federated secret stores | ✓ | ✓ | ✓ | ✓ |
| MCP endpoint | Shared | Shared | Scoped per agent | Scoped per agent |
| SSO with role mapping | - | ✓ | ✓ | ✓ |
| Private plugin registry | - | ✓ | ✓ | ✓ |
| Your own domain | - | - | ✓ | ✓ |
| Point-in-time recovery | - | - | ✓ | ✓ |
| Per-agent audit trail, SIEM export | - | - | - | ✓ |
| VPC peering, data residency | - | - | - | ✓ |
| Custom backup retention | - | - | - | ✓ |
| Replicas | 1 | 2, up to 4 | 2, up to 10 | 3, up to 20 |
| Database storage | 5 GiB | 50 GiB | 200 GiB | 500 GiB |
| Support | Community | Email | Priority, shared Slack | Named contact |
| Uptime SLA | - | 99.5% | 99.9% | 99.95% |

Free is a plan, not a trial: no card, no end date, every plugin and federated secret stores included. Enterprise is contracted, for more than one instance, an unlisted region, data residency or your own terms. [Talk to us](mailto:support@marmotdata.io).

The asset limit is the one you meet first, and one Postgres pipeline against a warehouse-sized database can produce thousands of assets. Scope each pipeline's include patterns to the schemas people ask about. The lookup limit is about agents, not people: a dozen agents fanning out across the catalog will reach it, a team browsing the UI never will.

A Free instance runs one replica, so an upgrade or node maintenance is a brief outage. From Team upward an instance runs at least two and scales out under load.

## Sizes

| Size | vCPU | Memory | Billing |
| --- | --- | --- | --- |
| Standard | 1 | 2 GB | Included |
| Performance | 2 | 8 GB | Add-on |
| Max | 4 | 16 GB | Add-on |

Performance and Max appear on your subscription as quantity line items, recalculated whenever an instance is created or deleted. Billing shows the resulting monthly total immediately.

<TipBox variant="warning" title="A stopped instance is still billed">
Stopping takes an instance out of service but keeps its slot and its size add-on. Deleting is what removes it, and deleting is irreversible, so export first.
</TipBox>

## Changing plan

Upgrading takes effect immediately: add a card, choose the plan, and the capabilities unlock without touching the running instance. Downgrading to Free cancels the subscription; the console handles it.

Card details go straight to Stripe. Invoices are under Billing with a PDF each, and company details and VAT number entered under Account appear on every invoice from then on. Manage in Stripe opens the Stripe portal for billing contacts and cancellation, and a cancellation made there is reflected back automatically.

A failed instance is never billed. A [degraded](instances.md#lifecycle) one is billed normally while the platform restores it.

<CalloutCard
  title="Not sure which plan fits?"
  description="Tell us what you want to catalog and how many agents will query it, and we will tell you honestly, including when the answer is Free."
  href="mailto:support@marmotdata.io"
  buttonText="Contact us"
  variant="secondary"
  icon="mdi:email-fast"
/>
