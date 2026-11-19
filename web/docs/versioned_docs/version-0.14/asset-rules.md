---
sidebar_position: 6
title: Asset rules
---

import { ThemedImg } from '@site/src/components/ThemedImg';

# Asset rules

An asset rule adds glossary terms and links to every asset that matches a search, and keeps doing it as the catalog changes. Assets that stop matching lose them again, so "every finance table links to the payments runbook" stays true without anyone maintaining it.

## Create a rule

1. Open **Asset rules** in the sidebar and select **Create asset rule**.
2. Enter a **Name** and, if it helps, a **Description**.
3. Under **Which assets**, write the search, or select **Build query** to pick fields. **Assets that match** lists what the rule will reach as you type.
4. Under **What to add**, add links (a title and a URL) and **Glossary terms**.
5. Select **Create asset rule**.

A rule that links a runbook to everything tagged `finance`:

<ThemedImg
  lightSrc="/img/asset-rules-query-light.png"
  darkSrc="/img/asset-rules-query-dark.png"
  alt="Create asset rule with the name Payments runbook, the search @tag: finance, a runbook link and the 9 assets that match"
  size="panel"
/>

The search uses the [query language](queries.md), the same as **Discover**. For example:

- `@type: "table"` matches every table.
- `@provider: "PostgreSQL"` matches everything discovered from PostgreSQL. Values match as stored, so use the provider name exactly as the catalog shows it.
- `@tag: "gdpr"` matches every asset tagged `gdpr`.
- `@metadata.owner: "Payments"` matches assets whose `owner` metadata field names that team.

## How rules are applied

A rule's terms appear under **What it means** on each matching asset, marked as applied by a rule, and its links appear with the asset's other links. When several rules match the same asset, it gets everything they add.

Marmot applies rules when assets are added and every 30 minutes. Turning off **Apply this rule** keeps the rule's settings but adds nothing until you turn it on again.

## Manage rules

**Asset rules** lists every rule with what it matches, what it adds and how many assets it reaches, with a switch to turn each one on or off:

<ThemedImg
  lightSrc="/img/asset-rules-list-light.png"
  darkSrc="/img/asset-rules-list-dark.png"
  alt="Asset rules listing Erasure scope, which matches assets tagged gdpr, adds the Erasure request term and an erasure runbook and reaches 4 assets"
  size="panel"
/>

Select a rule to edit it, see the assets it matches or delete it.

## Next steps

- [Glossary](glossary.md): the terms a rule can add.
- [Data products](data-products.md): group matching assets instead of enriching them.
