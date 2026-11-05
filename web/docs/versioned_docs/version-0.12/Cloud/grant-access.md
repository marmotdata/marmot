---
sidebar_position: 4.3
title: Grant and check access
description: Grant roles on Marmot Cloud from the UI or Terraform, set the base role and check what a person, team or agent can reach.
---

import Tabs from '@theme/Tabs';
import TabItem from '@theme/TabItem';
import { ThemedImg } from '@site/src/components/ThemedImg';
import { Steps, Step } from '@site/src/components/Steps';

# Grant and check access

Grant a role to a person, team or service account on one resource, then check what they can reach. [Access control](access-control.md) explains how grants inherit and add up.

## Before you start

- You hold `iam.admin` or `catalog.accessAdmin` on the resource or a wider scope.
- The member exists: a person who has signed in, a team or a [service account](service-accounts.md).

## Grant a role

Grant the role on the resource the member should reach, or across the whole catalog:

<Tabs groupId="method" queryString>
  <TabItem value="ui" label="UI" default>

1. Open the resource's access panel: the **Access** tab on an asset or data product, **Manage access** on a glossary term, or **Settings → Catalog-wide access** for the whole catalog.
2. Select **Grant access**.
3. Choose the member and the **Role**. Only roles that take effect on this resource are listed.
4. Optional: set **Access ends** and a **Reason** for access that's only needed for a while.
5. Select **Grant access**.

Granting the Trust & Safety team `dataProduct.viewer` on Buyer 360 for 30 days:

<ThemedImg
  lightSrc="/img/cloud-grant-access-light.png"
  darkSrc="/img/cloud-grant-access-dark.png"
  alt="The Grant access drawer on the Buyer 360 data product, with the Trust & Safety team, the dataProduct.viewer role, access ending in 30 days and a reason filled in"
  size="dialog"
/>

Secret stores and queryable pipelines have the same panel on their own pages.

  </TabItem>
  <TabItem value="terraform" label="Terraform">

```hcl
resource "marmot_data_product_iam_member" "trust_reads_buyer_360" {
  data_product_id = marmot_data_product.buyer_360.id
  role            = "dataProduct.viewer"
  member          = "group:${marmot_team.trust_and_safety.id}"
}
```

The same `_member` resource exists for `organization` (the whole catalog), `asset`, `glossary_term` and `secret_store`. Each adds one grant and leaves the rest alone, so it's safe alongside grants made in the UI. For a grant with an end date, use the UI.

The `_binding` and `_policy` resources own a whole role or policy instead, as the provider's [access control guide](https://registry.terraform.io/providers/marmotdata/marmot/latest/docs/guides/access-control) explains.

  </TabItem>
</Tabs>

## Set the base role

The base role is what everyone gets before you grant them anything. It starts as `user`, which reads the whole catalog. If the catalog holds something not everyone should read, lower it. Do it early, because a new base role only applies to people who haven't signed in yet.

<Steps>
<Step title="Give new people no access by default">

Open **Settings → Authentication** and, under **New user defaults**, set **Role assigned on first sign-in** to `catalog.none`:

<ThemedImg
  lightSrc="/img/cloud-new-user-defaults-light.png"
  darkSrc="/img/cloud-new-user-defaults-dark.png"
  alt="New user defaults in Settings, Authentication, with Role assigned on first sign-in set to catalog.none"
  size="panel"
/>

</Step>
<Step title="Remove the grant to everyone">

Open **Settings → Catalog-wide access**, open the menu on the **Everyone signed in** row and select **Revoke access**:

<ThemedImg
  lightSrc="/img/cloud-catalog-wide-access-light.png"
  darkSrc="/img/cloud-catalog-wide-access-dark.png"
  alt="Catalog-wide grants in Settings, listing Everyone signed in and three service accounts with their roles"
/>

</Step>
<Step title="Narrow existing members">

Under **Settings → Members**, change anyone who holds `user` to a narrower role.

</Step>
<Step title="Grant teams what they need">

[Grant each team a role](#grant-a-role) on the data products or assets it works with.

</Step>
</Steps>

## Check who has access

### One resource

Open the resource's **Access** tab, select **Check access** and choose a member. Marmot says whether they can see the resource and which grants give them access:

<ThemedImg
  lightSrc="/img/cloud-check-access-light.png"
  darkSrc="/img/cloud-check-access-dark.png"
  alt="Check access on the Buyer 360 data product, answering yes for Priya Shah and listing the grants that give her access"
  size="dialog"
/>

### One member

Check everything that reaches a member before you hand out a credential. **Effective access** is on each service account's and team's page in **Settings**, on a person under **Settings → Members** and under **Explain access** in **Settings → Catalog-wide access**:

<ThemedImg
  lightSrc="/img/cloud-effective-access-light.png"
  darkSrc="/img/cloud-effective-access-dark.png"
  alt="Effective access on a service account, listing the roles it holds across the catalog and the permissions each one carries"
  size="dialog"
/>

### Script an access review

The API is the only way to script this. Checking another member needs `iam:view` across the whole catalog:

```bash
export MARMOT_HOST=https://<instance>.marmotdata.cloud

curl -s -H "Authorization: Bearer $MARMOT_API_KEY" \
  "$MARMOT_HOST/api/v1/iam/effective-access?member=serviceAccount:$SA_ID"

curl -s -X POST -H "Authorization: Bearer $MARMOT_API_KEY" \
  -H 'Content-Type: application/json' \
  -d '{"permissions":["dataProduct:view"],"member":"user:'"$USER_ID"'"}' \
  "$MARMOT_HOST/api/v1/iam/dataProduct/$PRODUCT_ID/test-permissions"
```

`test-permissions` returns the permissions from your list that the member holds. Leave out `member` to test yourself.

## Troubleshooting

| What you see | What to do |
| --- | --- |
| **Check access** says access comes "through a grant to everyone signed in" | The base role is too wide. [Set the base role](#set-the-base-role). |
| The role you want isn't in the **Role** list | It only takes effect catalog-wide. Grant it under **Settings → Catalog-wide access**. |
| "Testing another member's access requires iam:view" | Ask for `iam.viewer` across the whole catalog. |
| "Someone else changed this policy while you were editing. Reloaded; try again." | Make the grant again on the reloaded policy. |

## Next steps

- [Set up service accounts](service-accounts.md): give an agent or a CI job its own identity to grant to.
- [Set up single sign-on](single-sign-on.md): fill teams from your directory so you grant to teams, not people.
