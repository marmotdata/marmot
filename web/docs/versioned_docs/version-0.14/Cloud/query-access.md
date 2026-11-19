---
sidebar_position: 4.4
title: Control who can query data
description: Choose which sources answer queries on Marmot Cloud, grant query access on a source, data product or table, and watch for unusual activity.
---

import Tabs from '@theme/Tabs';
import TabItem from '@theme/TabItem';
import { ThemedImg } from '@site/src/components/ThemedImg';

# Control who can query data

On Marmot Cloud, a person or agent can only run a [data query](../data-queries.md) against tables you've granted them. A statement runs only when every table it reads is covered by one of their grants, so an agent given one data product can't wander into the rest of a database.

## Before you start

- You hold `access.admin` across the whole catalog to choose which sources answer queries.
- You can grant access on the resource: `iam.admin` or `catalog.accessAdmin` there, or across the whole catalog. Grants on a source need someone who manages access across the whole catalog.

## Let a source answer queries

1. Open **Plugins**, select the pipeline and select **Edit pipeline**.
2. Under **Answer queries**, turn on **Let agents and people query this source**.
3. Optional: turn on **Refresh the catalog when queried** to refresh a table's details, at most hourly, whenever a query reads it.
4. Select **Save changes**.

The source's page then lists who can query it under **Granted on this source**:

<ThemedImg
  lightSrc="/img/cloud-answer-queries-light.png"
  darkSrc="/img/cloud-answer-queries-dark.png"
  alt="The Answer queries section of a pipeline with Let agents and people query this source turned on, and the grants made on the orders-db source below it"
  size="panel"
/>

Admins and anyone holding `asset.dataQuerier` across the whole catalog can already query every source that answers.

## Grant query access

Grant `asset.dataQuerier` on the narrowest resource that covers what the member needs:

- **A source** covers every table it reads, including tables it hasn't catalogued, so statements Marmot can't fully plan are allowed too.
- **A data product** covers the tables in it.
- **An asset** covers that one table.

<Tabs groupId="method" queryString>
  <TabItem value="ui" label="UI" default>

1. Open the resource: a source under **Plugins → Edit pipeline**, or a data product or asset's **Access** tab.
2. Select **Grant access**.
3. Choose the **Member** and the role `asset.dataQuerier`.
4. Check **Queryable tables covered**, which lists what the grant would reach.
5. Optional: set **Access ends** and a **Reason**.
6. Select **Grant access**.

The drawer for a 30-day grant on the `orders-db` source:

<ThemedImg
  lightSrc="/img/cloud-query-grant-light.png"
  darkSrc="/img/cloud-query-grant-dark.png"
  alt="The Grant access drawer for the orders-db source with Leo Martin, the asset.dataQuerier role, 4 queryable tables covered, access ending in 30 days and a reason"
  size="dialog"
/>

  </TabItem>
  <TabItem value="terraform" label="Terraform">

Terraform grants on a data product or an asset. It can't grant on a source or set an end date; use the UI for those.

```hcl
resource "marmot_data_product_iam_member" "support_queries_buyers" {
  data_product_id = marmot_data_product.buyer_360.id
  role            = "asset.dataQuerier"
  member          = "serviceAccount:${marmot_service_account.support_triage.id}"
}
```

  </TabItem>
</Tabs>

`asset.dataQuerier` doesn't include seeing the pipeline itself, so a member granted only on a source can query it without seeing its configuration.

## See who can query

**Data access** in the sidebar lists everyone who may run queries, what each grant is on and when it ends. It needs `iam:view` across the whole catalog. Grants across the whole catalog are under **Settings → Catalog-wide access**.

<ThemedImg
  lightSrc="/img/cloud-data-access-light.png"
  darkSrc="/img/cloud-data-access-dark.png"
  alt="The Data access page listing query grants: support-triage-agent on the Buyer 360 data product, the Analytics Engineering team and two people on the orders-db source, one ending soon"
  size="panel"
/>

To extend or shorten a grant, select **Change expiry** on its row on the resource. Seven days before a grant ends, its holder and the people who manage access on the resource are notified. A grant that has ended is removed 30 days later.

## What members see

In **Query**, **My access** lists where the signed-in member can run queries and when each grant ends. When a query is refused, **Request access** notifies the people who can grant what it needed, and every refusal is recorded in [Query audit](../query-audit.md).

## Who can review queries

Two roles cover the query log and its insights, and both are granted across the whole catalog:

- `access.viewer` sees **Query audit**, the data queries view of **Insights** and plugin status.
- `access.admin` can also choose which sources answer queries.

## Watch for unusual activity

**Insights → Data queries** opens with **Unusual activity** for anyone holding `access.viewer` or `access.admin`:

<ThemedImg
  lightSrc="/img/cloud-unusual-activity-light.png"
  darkSrc="/img/cloud-unusual-activity-dark.png"
  alt="The Unusual activity table on Insights with a critical spike in denied queries, a warning for failing queries on orders-db and several new callers"
  size="panel"
/>

| What happened | Severity |
| --- | --- |
| **New caller**: someone queries for the first time | For information |
| **First read of a table**: an established caller reads a table they haven't before | For information, or Warning when the table is tagged `pii`, `sensitive`, `confidential` or `restricted` |
| **More failures than usual**: queries failing at the source | Warning |
| **More denials than usual**: queries refused by access rules | Warning, or Critical at 15 or more in two hours |

A spike compares the last two hours with the usual for the same span over the previous 7 days, and needs at least 5 queries and three times the usual to show. The rest of Insights works as it does in [open source Marmot](../metrics.md).

## Next steps

- [Query your data](../query-data.md): run queries, save them and export results.
- [Query audit](../query-audit.md): every query, who ran it and why it was allowed or refused.
- [Grant and check access](grant-access.md): the same grants for catalog roles.
