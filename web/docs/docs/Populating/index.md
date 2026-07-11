# Populating Your Catalog

There are many ways to populate Marmot with assets and lineage. Use all of these methods together, or just the ones that fit your existing workflows.

import { CalloutCard, DocCard, DocCardGrid } from '@site/src/components/DocCard';

<CalloutCard
  title="Auto-Discover with Plugins"
  description="The easiest way to get started - automatically discover and catalog assets from your data sources."
  docId="Plugins/index"
  buttonText="Browse Plugins"
  icon="mdi:puzzle"
/>

## Methods

<DocCardGrid>
  <DocCard
    title="UI"
    description="Manually add and manage assets directly in the Marmot web interface"
    docId="Populating/UI"
    icon="mdi:cursor-default-click"
  />
  <DocCard
    title="CLI"
    description="Simple YAML configuration for fetching assets from various data sources"
    docId="Populating/CLI"
    icon="mdi:console"
  />
  <DocCard
    title="Terraform"
    description="Integrate with your existing IaC pipelines using the Marmot provider"
    docId="Populating/Terraform"
    icon="mdi:terraform"
  />
  <DocCard
    title="Pulumi"
    description="Build your catalog with your favourite programming language"
    docId="Populating/Pulumi"
    icon="mdi:code-braces"
  />
  <DocCard
    title="REST API"
    description="Custom integrations with your existing tooling or software"
    docId="Populating/API"
    icon="mdi:api"
  />
  <DocCard
    title="Kubernetes Operator"
    description="Ingest assets on a schedule with declarative Kubernetes resources"
    docId="Populating/Operator"
    icon="mdi:kubernetes"
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
