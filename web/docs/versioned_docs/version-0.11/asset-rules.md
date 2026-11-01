---
sidebar_position: 6
---

import { CalloutCard } from '@site/src/components/DocCard';
import { ThemedImg } from '@site/src/components/ThemedImg';

# Asset Rules

Asset Rules automatically apply enrichments to assets matching specific criteria. Define a rule once and Marmot keeps everything in sync as your Catalog changes, including new assets that match.

<CalloutCard
  title="Get Started in Five Minutes"
  description="Follow the Quick Start Guide to try Marmot out locally."
  docId="quick-start"
  buttonText="Quick Start"
  icon="mdi:rocket-launch"
/>

## Creating a Rule

Navigate to **Asset Rules** under **Governance** in the header and click **Create Rule**. The creation flow has three steps.

### Basic Information

Give your rule a unique name and an optional description.

<ThemedImg
  lightSrc="/img/asset-rules-basicinfo-light.png"
  darkSrc="/img/asset-rules-basicinfo-dark.png"
  alt="Step one of the asset rule wizard, with a name and description"
/>

### Enrichments

Choose what to apply to matching assets. A rule must include at least one of:

- **External links**: runbooks, dashboards, wiki pages or monitoring URLs. Each link has a name, URL and optional icon.
- **Glossary terms**: select one or more terms from your existing glossary to associate with matching assets.

<ThemedImg
  lightSrc="/img/asset-rules-enrichments-light.png"
  darkSrc="/img/asset-rules-enrichments-dark.png"
  alt="Step two of the asset rule wizard, with an external link and a glossary term attached"
/>

### Query

Define which assets the rule should match using Marmot's query language (the same syntax used in search). For example:

- `@type: "table"` matches every table in the catalog
- `@provider: "PostgreSQL"` matches everything discovered from PostgreSQL
- `@metadata.owner: "platform-team"` matches assets whose `owner` metadata field names that team

The fields are the ones the [query language](queries.md) supports: `@type`, `@provider`, `@name`, `@kind` and `@metadata.*`. Values are matched as stored, so use the provider name exactly as it appears in the catalog.

Use the **Preview** button to see which assets currently match before saving.

<ThemedImg
  lightSrc="/img/asset-rules-query-light.png"
  darkSrc="/img/asset-rules-query-dark.png"
  alt="Step three of the asset rule wizard, previewing how many assets the query matches"
/>

## How Rules Are Applied

Once a rule matches, its enrichments show up on the asset itself: external links beside the title, glossary terms in the sidebar.

<ThemedImg
  lightSrc="/img/glossary-on-asset-light.png"
  darkSrc="/img/glossary-on-asset-dark.png"
  alt="An asset page showing the runbook link and the glossary term that an asset rule attached"
/>

Rules are evaluated every 30 minutes by default and whenever a new asset is added to the Catalog. Only rules whose configuration or matching assets have changed are re-evaluated. When multiple rules match the same asset, all enrichments are applied.

Rules can be enabled or disabled at any time. Disabled rules retain their configuration so you can re-enable them later.

## Managing Rules

<ThemedImg
  lightSrc="/img/asset-rules-list-light.png"
  darkSrc="/img/asset-rules-list-dark.png"
  alt="The Asset Rules page listing rules with their match counts and status"
/>

The Asset Rules page lists all rules with their match count, number of links and terms, enabled status and last updated time. Click any rule to view its configuration or see matched assets.

Rules can be edited, enabled, disabled or deleted from the detail page. Changes take effect on the next reconciliation cycle, or you can trigger an immediate evaluation by updating the rule.

<CalloutCard
  title="Need Help?"
  description="Join the Discord community to ask questions and share how you're using Asset Rules."
  href="https://discord.gg/TWCk7hVFN4"
  buttonText="Join Discord"
  variant="secondary"
  icon="mdi:account-group"
/>
