---
sidebar_position: 4
title: Access control
description: How access works on Marmot Cloud. Roles granted on the whole catalog, a data product, an asset, a glossary term, a secret store or a source, inherited downwards and added together.
---

import { ThemedImg } from '@site/src/components/ThemedImg';

# Access control

On Marmot Cloud you grant a role to a person, team or service account on one resource, so an agent's service account can be granted the 3 assets it should see and reach nothing else.


## How it works

A grant reaches the resource it's made on and everything under it. A source is a pipeline that answers [data queries](../data-queries.md), and its tables sit under it:

<figure className="doc-diagram" role="img" aria-label="The whole catalog contains data products, assets, glossary terms, secret stores and sources. A data product contains its assets, a glossary term contains its child terms and a source contains its tables. A grant covers everything below the resource it's made on.">
  <svg viewBox="0 0 640 250" xmlns="http://www.w3.org/2000/svg">
    <rect className="doc-diagram__box doc-diagram__box--root" x="230" y="10" width="180" height="40" rx="6" />
    <text className="doc-diagram__label" x="320" y="35" textAnchor="middle">Whole catalog</text>
    <path className="doc-diagram__line" d="M320 50 V80 M64 80 H576 M64 80 V100 M192 80 V100 M320 80 V100 M448 80 V100 M576 80 V100" />
    <rect className="doc-diagram__box" x="8" y="100" width="112" height="40" rx="6" />
    <text className="doc-diagram__label" x="64" y="125" textAnchor="middle">Data product</text>
    <rect className="doc-diagram__box" x="136" y="100" width="112" height="40" rx="6" />
    <text className="doc-diagram__label" x="192" y="125" textAnchor="middle">Asset</text>
    <rect className="doc-diagram__box" x="264" y="100" width="112" height="40" rx="6" />
    <text className="doc-diagram__label" x="320" y="125" textAnchor="middle">Glossary term</text>
    <rect className="doc-diagram__box" x="392" y="100" width="112" height="40" rx="6" />
    <text className="doc-diagram__label" x="448" y="125" textAnchor="middle">Secret store</text>
    <rect className="doc-diagram__box" x="520" y="100" width="112" height="40" rx="6" />
    <text className="doc-diagram__label" x="576" y="125" textAnchor="middle">Source</text>
    <path className="doc-diagram__line" d="M64 140 V190 M320 140 V190 M576 140 V190" />
    <rect className="doc-diagram__box doc-diagram__box--child" x="8" y="190" width="112" height="40" rx="6" />
    <text className="doc-diagram__label doc-diagram__label--child" x="64" y="215" textAnchor="middle">Its assets</text>
    <rect className="doc-diagram__box doc-diagram__box--child" x="264" y="190" width="112" height="40" rx="6" />
    <text className="doc-diagram__label doc-diagram__label--child" x="320" y="215" textAnchor="middle">Child terms</text>
    <rect className="doc-diagram__box doc-diagram__box--child" x="520" y="190" width="112" height="40" rx="6" />
    <text className="doc-diagram__label doc-diagram__label--child" x="576" y="215" textAnchor="middle">Its tables</text>
  </svg>
</figure>

- **Grants inherit downwards.** A grant on the whole catalog covers everything. A grant on a data product covers its assets, including ones that match its [rules](/docs/data-products) later. A grant on a glossary term covers the terms under it, and a grant on a source covers every table it serves.
- **Grants add up.** There are no deny rules, so keep catalog-wide grants narrow and [lower the base role](grant-access.md#set-the-base-role) to keep something from most people.
- **Grants can end.** A grant with an end date stops working then. Marmot warns the holder and the people who manage access on the resource 7 days before, and removes the ended grant 30 days later.

Roles assigned under **Settings → Members** or on a service account, including the base role a new user gets, are catalog-wide grants.

## Members

A member is `user:<id>`, `group:<team-id>`, `serviceAccount:<id>` or `allAuthenticated` (everyone who can sign in). Grant to teams rather than people, and let [single sign-on](single-sign-on.md) keep teams in step with your directory.

## Roles

Write permissions are checked across the whole catalog, so editing and administration roles only take effect there. These roles can also be granted lower down:

| Role | Gives | Grant on |
| --- | --- | --- |
| `catalog.viewer` | Read assets, data products and the glossary. | Data product, asset, glossary term |
| `catalog.accessAdmin` | `catalog.viewer`, plus deciding who else has access. | Data product, asset, glossary term |
| `iam.viewer`, `iam.admin` | Read, or change, who has access. | Data product, asset, glossary term |
| `asset.viewer` | Assets and their lineage neighbours, without data. | Data product, asset |
| `asset.dataQuerier` | `asset.viewer`, plus read-only queries. | Data product, asset, source |
| `dataProduct.viewer` | A data product and its assets. | Data product |
| `glossary.viewer` | Glossary terms. | Glossary term |
| `secretStore.viewer` | A store and its secrets, without values. | Secret store |
| `secretStore.user` | `secretStore.viewer`, plus attaching secrets to pipelines. | Secret store |
| `secretStore.reader` | `secretStore.viewer`, plus reading values. | Secret store |

Every role can be granted across the whole catalog, including `admin`, `editor` (create and edit catalog content and pipelines), `user` (read the catalog, the base role for new users), `access.viewer` (the query log and query insights), `access.admin` (also choosing which sources answer queries) and `catalog.none` (nothing). [Control who can query data](query-access.md) covers the query roles.

Grant `asset.viewer` widely and `asset.dataQuerier` narrowly. Pipeline authors need `secretStore.user`, and almost nobody needs `secretStore.reader` or `secretStore.admin`, which can both read values.

**Settings → Roles** lists every role and the permissions it carries:

<ThemedImg
  lightSrc="/img/cloud-admin-roles-light.png"
  darkSrc="/img/cloud-admin-roles-dark.png"
  alt="Settings, Roles in a Marmot Cloud instance, listing the granular roles with the permissions each one carries"
/>

## Who has access

The **Access** tab on a data product or asset lists what's granted on it and what reaches it from a wider scope:

<ThemedImg
  lightSrc="/img/cloud-access-panel-light.png"
  darkSrc="/img/cloud-access-panel-dark.png"
  alt="The Access tab of a data product, showing nothing granted on the product itself and grants that reach it from across the catalog"
/>

To explain one member's access, use [Check access](grant-access.md#check-who-has-access).

## Limitations

- You can't let a team edit only one data product's assets.
- Only someone who manages access across the whole catalog can grant on a secret store or a source.
- The Terraform provider can't grant on a source.

## Next steps

- [Grant and check access](grant-access.md): give a team or an agent a role on a resource.
- [Set up service accounts](service-accounts.md): identities for agents, scripts and CI.
- [Set up single sign-on](single-sign-on.md): keep teams in step with your directory.
