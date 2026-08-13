---
slug: import-openmetadata-into-marmot
title: "Migrate from OpenMetadata to Marmot in five minutes"
authors:
  - name: Bruno Schaatsbergen
    url: https://github.com/bschaatsbergen
image: /img/marmot-openmetadata-banner.png
description: "Marmot's new OpenMetadata plugin imports an entire OpenMetadata instance in one run: tables, topics, dashboards, pipelines, lineage and the business glossary. Five minutes of setup, and it keeps syncing until the day you switch OpenMetadata off. This post is the migration, start to finish: run both catalogs while you move, adopt native plugins one system at a time, no big bang required."
tags: [openmetadata, migration, data-catalog, data-discovery]
keywords:
  - openmetadata alternative
  - migrate from openmetadata
  - openmetadata to marmot
  - import openmetadata catalog
  - openmetadata migration
---

import { ThemedImg } from '@site/src/components/ThemedImg';
import { CalloutCard } from '@site/src/components/DocCard';

<div style={{textAlign: 'center', marginBottom: '2rem'}}>
  <img src="/img/marmot-openmetadata-banner.png" alt="Marmot importing an OpenMetadata catalog" style={{maxWidth: '100%', borderRadius: '8px'}} />
</div>

This one is for everyone running OpenMetadata and curious about Marmot. The new [OpenMetadata plugin](/docs/Plugins/OpenMetadata) imports your entire instance in one run, about five minutes of setup, and keeps syncing from OpenMetadata until the day you switch it off. And this is where we get to walk the walk: Marmot runs for free what OpenMetadata makes you pay to keep running. The search cluster, the Airflow deployment, the fleet of services behind the catalog: gone. One Go binary on Postgres, and trying it costs you nothing. No big bang, no re-cataloging, nothing to undo if you walk away.

We can make that promise because of what a catalog actually is. The most valuable thing in one is not the software, it is the years of curation inside it: the descriptions, the owners, the glossary the business argued over, the lineage. That work is yours, and it should be portable. This plugin is us acting like it.

This post is the migration, start to finish.

<!-- truncate -->

## One run, your whole catalog

Point the plugin at your OpenMetadata host and one run imports tables, topics, buckets, dashboards, pipelines, ML models, API endpoints, the business glossary, lineage and recent pipeline executions. Every entity lands as the technology it actually describes: a table under a Postgres service becomes a PostgreSQL asset, addressed exactly as Marmot's own PostgreSQL plugin would address it. Technologies Marmot has no plugin for yet, such as Snowflake or Looker, come across under their own provider name. The result looks like a catalog Marmot built itself.

<div style={{textAlign: 'center', margin: '2rem 0'}}>
  <ThemedImg
    lightSrc="/img/marmot-openmetadata-assets-light.png"
    darkSrc="/img/marmot-openmetadata-assets-dark.png"
    alt="Assets imported from OpenMetadata in Marmot, grouped by the technology they belong to"
  />
</div>

Descriptions, columns, tags, owners and domains come across on each asset, and nothing loses its trail. Every asset gets an OpenMetadata link that jumps straight to the entity it was imported from, and an `openmetadata` metadata object carrying the fully qualified name, the service and when the entity last changed, so mid-migration any asset in Marmot traces back to its source in one click. The glossary stays a first-class glossary rather than being flattened into tags, and lineage keeps the pipeline that moved the data on each edge. The [plugin docs](/docs/Plugins/OpenMetadata) hold the exact entity-by-entity mapping.

<div style={{textAlign: 'center', margin: '2rem 0'}}>
  <ThemedImg
    lightSrc="/img/marmot-openmetadata-asset-light.png"
    darkSrc="/img/marmot-openmetadata-asset-dark.png"
    alt="An imported table in Marmot carrying its OpenMetadata descriptions, tags and metadata"
  />
</div>

---

## The migration

### 1. Schedule the import

Grab a token in OpenMetadata under **Settings**, then **Bots**, and set the import up as a recurring pipeline, for example with the [Terraform provider](https://registry.terraform.io/providers/marmotdata/marmot/latest/docs):

```hcl
resource "marmot_pipeline" "openmetadata" {
  name      = "openmetadata-import"
  plugin_id = "openmetadata"

  config = jsonencode({
    host      = "https://openmetadata.company.com"
    jwt_token = # inject securely
  })

  cron_expression = "0 * * * *" # hourly
}
```

Everything is imported by default; the [configuration reference](/docs/Plugins/OpenMetadata) covers scoping down to specific services, and the same config works from the UI wizard, CLI, Pulumi and the REST API.

### 2. Keep working in both catalogs

Each scheduled run brings across whatever changed in OpenMetadata, so the two stay in step for as long as the move takes. Re-running is safe, and anything written in Marmot survives every re-sync: an edited description is stored separately from the imported one, so the next run refreshes the imported side without touching the edit. The same holds for tags, owners and glossary terms added in Marmot. Curation never pauses for the migration.

### 3. Adopt native plugins one system at a time

When you are ready to catalog a system directly, add its own pipeline, for example the [PostgreSQL plugin](/docs/Plugins/PostgreSQL) against the database OpenMetadata was describing. Imported assets carry the same identity the native plugin uses, so the native run takes over the existing assets instead of creating a second copy. Nothing gets deleted or re-pointed, and the descriptions people wrote stay put. Work through your systems at whatever pace suits.

### 4. Switch OpenMetadata off

When nothing depends on it anymore, stop scheduling the run. The imported assets stay exactly as they are; there is no cliff on the day you turn it off.

One warning for that last day: retire the plugin by removing its schedule, not with `marmot ingest --destroy`. Destroy deletes every asset the pipeline ever created, including ones a native pipeline has since taken over.

---

## What you end up with

The whole catalog in one graph, whichever system each piece came from. Here the path OpenMetadata held, a PostgreSQL table and a Kafka topic feeding a Snowflake mart feeding a Looker dashboard, survives the import intact:

<div style={{textAlign: 'center', margin: '2rem 0'}}>
  <ThemedImg
    lightSrc="/img/marmot-openmetadata-lineage-light.png"
    darkSrc="/img/marmot-openmetadata-lineage-dark.png"
    alt="Cross-technology lineage imported from OpenMetadata rendered in Marmot"
  />
</div>

And the glossary your business spent months agreeing on, with definitions, synonyms and assignments intact:

<div style={{textAlign: 'center', margin: '2rem 0'}}>
  <ThemedImg
    lightSrc="/img/marmot-openmetadata-glossary-light.png"
    darkSrc="/img/marmot-openmetadata-glossary-dark.png"
    alt="A business glossary imported from OpenMetadata into Marmot"
  />
</div>

All of it runs on one Go binary and a Postgres database, browsable in the UI and served to Claude, Cursor or any other assistant over [MCP](/docs/MCP/). If you are still weighing the move itself, the [full comparison](/resources/marmot-vs-openmetadata) covers footprint, MCP and connectors line by line.

The plugin is experimental for now. If you run it against a real OpenMetadata instance, I want to hear what came across wrong and what got skipped that should not have been. The fastest way to reach us is Discord.

<CalloutCard
  title="Join the Community"
  description="Get help, share feedback and connect with other Marmot users on Discord."
  href="https://discord.gg/TWCk7hVFN4"
  buttonText="Join Discord"
  variant="secondary"
  icon="mdi:account-group"
/>
