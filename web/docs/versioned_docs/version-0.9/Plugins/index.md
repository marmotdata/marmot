# Plugins

Plugins automatically discover and catalog your data assets in Marmot. They connect to external systems, extract metadata and lineage, and create asset entries with minimal effort.

Marmot isn't limited to plugin-based ingestion. You can also use:

### Infrastructure as Code

- [Terraform Provider](/docs/populating/terraform) - Manage Marmot assets as Terraform resources
- [Pulumi Package](/docs/populating/pulumi) - Integrate Marmot with Pulumi infrastructure definitions

These approaches enable version-controlled asset definitions and integration with existing infrastructure workflows.

### API

The [Marmot API](/docs/populating/api) lets you programmatically create, update, and manage assets.

## Available Plugins

import PluginCards from '@site/src/components/PluginCards';
import { CalloutCard } from '@site/src/components/DocCard';

<PluginCards />

<CalloutCard
  title="Don't see your data source?"
  description="Learn how to build a custom plugin to connect Marmot to any data source."
  href="/docs/Develop/creating-plugins"
  buttonText="Create a Plugin"
  variant="secondary"
  icon="mdi:puzzle-plus"
/>
