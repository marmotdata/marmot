# Populating Your Catalog

There are many ways to populate Marmot with assets and lineage. Use all of these methods together, or just the ones that fit your existing workflows.

import { CalloutCard, DocCard, DocCardGrid } from '@site/src/components/DocCard';

<CalloutCard
  title="Auto-Discover with Plugins"
  description="The easiest way to get started - automatically discover and catalog assets from your data sources."
  href="/docs/Plugins"
  buttonText="Browse Plugins"
  icon="mdi:puzzle"
/>

## Methods

<DocCardGrid>
  <DocCard
    title="CLI"
    description="Simple YAML configuration for fetching assets from various data sources"
    href="/docs/Populating/CLI"
    icon="mdi:console"
  />
  <DocCard
    title="Terraform"
    description="Integrate with your existing IaC pipelines using the Marmot provider"
    href="/docs/Populating/Terraform"
    icon="mdi:terraform"
  />
  <DocCard
    title="Pulumi"
    description="Build your catalog with your favourite programming language"
    href="/docs/Populating/Pulumi"
    icon="mdi:code-braces"
  />
  <DocCard
    title="REST API"
    description="Custom integrations with your existing tooling or software"
    href="/docs/Populating/API"
    icon="mdi:api"
  />
</DocCardGrid>

<CalloutCard
  title="Explore the API"
  description="View the full API documentation with interactive examples."
  href="/api"
  buttonText="View API Docs"
  variant="secondary"
  icon="mdi:book-open-variant"
/>
