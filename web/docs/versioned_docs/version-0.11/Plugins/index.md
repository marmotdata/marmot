# Plugins

Plugins automatically discover and catalog your data assets in Marmot. They connect to external systems, extract metadata and lineage, and create asset entries with minimal effort.

import { CalloutCard } from '@site/src/components/DocCard';

<CalloutCard
  title="Browse the plugin registry"
  description="See every available plugin, with configuration and usage for each, on the Marmot plugin registry."
  href="https://plugins.marmotdata.io"
  buttonText="View all plugins"
  variant="external"
  icon="mdi:puzzle"
/>

Marmot isn't limited to plugin-based ingestion. You can also use:

### Infrastructure as Code

- [Terraform Provider](/docs/Populating/Terraform) - Manage Marmot assets as Terraform resources
- [Pulumi Package](/docs/Populating/Pulumi) - Integrate Marmot with Pulumi infrastructure definitions

These approaches enable version-controlled asset definitions and integration with existing infrastructure workflows.

### API

The [Marmot API](/docs/Populating/API) lets you programmatically create, update, and manage assets.

<CalloutCard
  title="Don't see your data source?"
  description="Learn how to build a custom plugin to connect Marmot to any data source."
  docId="Develop/creating-plugins"
  buttonText="Create a Plugin"
  variant="secondary"
  icon="mdi:puzzle-plus"
/>
