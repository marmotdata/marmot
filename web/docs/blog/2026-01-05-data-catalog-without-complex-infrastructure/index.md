---
slug: data-catalog-without-complex-infrastructure
title: "Marmot: Data catalog without the complex infrastructure"
authors:
  - name: Charlie Haley
    url: https://github.com/charlie-haley
image: /img/marmot-banner.png
---

import { CalloutCard } from '@site/src/components/DocCard';

<div style={{textAlign: 'center', marginBottom: '2rem'}}>
  <img src="/img/marmot-banner.png" alt="Marmot" style={{maxWidth: '100%', borderRadius: '8px'}} />
</div>

Data catalogs shouldn't need an entire platform team to run them.

Marmot is an open source data catalog that just needs PostgreSQL - no Kafka, no Elasticsearch, no Airflow. A single binary, deployable in minutes, focused on simplicity for everyone.

<!-- truncate -->

<CalloutCard
  title="See Marmot in Action"
  description="Explore the interface and features with the interactive demo - no installation required."
  href="https://demo.marmotdata.io"
  buttonText="Try Live Demo"
  icon="mdi:rocket-launch"
/>

## Why Marmot?

Modern data stacks are fragmented. Assets live across vendors, warehouses, message queues, object storage and APIs. Existing open source catalogs can help - but they come with baggage.

Many require external orchestrators, search indexers and message brokers alongside the main application. That means more infrastructure to manage, more things to break and more complexity to debug.

- **PostgreSQL only** - no Elasticsearch, no message brokers, no graph databases
- **Go-based** - a single binary that runs comfortably on modest infrastructure
- **No orchestrator dependency** - ingestion runs directly from the UI or CLI
- **Terraform and Pulumi native** - infrastructure-as-code from day one
- **Custom query language** - find any asset with precise queries, not just basic filters
- **Open source** - MIT licence with enterprise features like SSO included

## Architecture

Marmot is built entirely in Go with PostgreSQL being the only external dependency, handling search, job scheduling and metadata storage. It's so lightweight that the public demo at [demo.marmotdata.io](https://demo.marmotdata.io) runs on a single $4/month cloud instance.

Unlike traditional catalogs that have opinionated ingestion methods, Marmot lets you populate your catalog however you like. The UI supports manual entries and automated discovery via the plugin system. The CLI uses the same plugin system, so you can run ingestion jobs from your Marmot instance or as part of your CI/CD pipelines. Terraform, Pulumi and the REST API are there for infrastructure-as-code workflows and custom integrations.

<div style={{textAlign: 'center', margin: '2rem 0'}}>
  <img src="/img/marmot-diagram.png" alt="Marmot architecture diagram" style={{maxWidth: '100%', borderRadius: '8px'}} />
</div>

## Discovery and Lineage

Plugins let you get started quickly. PostgreSQL, MySQL, MongoDB, ClickHouse, BigQuery, Kafka, S3, GCS, Azure Blob, Airflow, dbt and more - with the ecosystem growing. Simply fill out the configuration with the required fields and run the job via the UI. You can also run the same plugins via the CLI if you want to keep your ingestion jobs local to your data assets.

For everything else, Terraform, Pulumi and the REST API let you document almost anything.

<div style={{textAlign: 'center', margin: '2rem 0'}}>
  <img src="/img/marmot-plugins.png" alt="Marmot plugin selection" style={{maxWidth: '100%', borderRadius: '8px'}} />
</div>

Supported plugins such as Airflow and dbt will automatically fill out lineage for assets, allowing you to quickly see the flow of data between your assets. You can also manually link assets in the UI or wire them up with Terraform or Pulumi. You can even capture lineage automatically via OpenLineage.

<div style={{textAlign: 'center', margin: '2rem 0'}}>
  <img src="/img/marmot-lineage.png" alt="Marmot lineage visualisation" style={{maxWidth: '100%', borderRadius: '8px'}} />
</div>

<CalloutCard
  title="It's Easy to Build a Plugin"
  description="A plugin is just a Go package with a simple interface - if you can fetch metadata from a system, you can build one."
  href="/docs/Develop/creating-plugins"
  buttonText="Plugin Guide"
  icon="mdi:puzzle-plus"
  variant="secondary"
/>

## Search

Marmot has a custom query language that lets you search across everything - assets, glossary terms, data products - all in one place.

e.g Find all PostgreSQL tables owned by a specific team:

```
@provider = PostgreSQL AND @type = Table AND @metadata.owner = "Order Management Team"
```

Full-text search, metadata filters, boolean logic, wildcards and range queries. The query language is expressive enough to handle complex searches but simple enough that you'll pick it up in minutes.

<div style={{textAlign: 'center', margin: '2rem 0'}}>
  <img src="/img/marmot-search.png" alt="Marmot search interface" style={{maxWidth: '100%', borderRadius: '8px'}} />
</div>

## AI Integration

Marmot includes a built-in [Model Context Protocol (MCP)](/docs/MCP) server. This lets AI assistants query your catalog using natural language - ask questions like "what tables does the analytics team own?" or "show me the upstream dependencies for user_events" directly in your editor or chat interface.

Works with Claude Desktop, Claude Code, Cursor, Cline and other MCP-compatible tools.

## Try It Out

Marmot is still early and I'm actively looking for feedback to shape the roadmap. Get involved on GitHub, reach out on Discord or just try the demo and let me know what you think.

- **Live demo:** [demo.marmotdata.io](https://demo.marmotdata.io)
- **Documentation:** [marmotdata.io/docs](https://marmotdata.io/docs)
- **GitHub:** [github.com/marmotdata/marmot](https://github.com/marmotdata/marmot)

<CalloutCard
  title="Join the Community"
  description="Get help, share feedback and connect with other Marmot users on Discord."
  href="https://discord.gg/TWCk7hVFN4"
  buttonText="Join Discord"
  variant="secondary"
  icon="mdi:account-group"
/>
