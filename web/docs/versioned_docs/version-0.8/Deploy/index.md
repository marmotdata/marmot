# Deploy

There are multiple ways to deploy Marmot - choose whichever method works best with your existing infrastructure and workflows.

import { CalloutCard, DocCard, DocCardGrid } from '@site/src/components/DocCard';

<CalloutCard
  title="Try Before You Deploy"
  description="Explore Marmot's features with our live demo - no installation required."
  href="https://demo.marmotdata.io"
  buttonText="View Live Demo"
  icon="mdi:rocket-launch"
/>

## Deployment Options

<DocCardGrid>
  <DocCard
    title="Docker Compose"
    description="Deploy Marmot and PostgreSQL together with one command"
    href="/docs/Deploy/Docker-Compose"
    icon="mdi:docker"
  />
  <DocCard
    title="Docker"
    description="Deploy using containers with your own PostgreSQL"
    href="/docs/Deploy/Docker"
    icon="mdi:docker"
  />
  <DocCard
    title="Helm / Kubernetes"
    description="Deploy to Kubernetes clusters with the official Helm chart"
    href="/docs/Deploy/Helm"
    icon="mdi:kubernetes"
  />
  <DocCard
    title="CLI / Binary"
    description="Run directly on your system with the single binary"
    href="/docs/Deploy/CLI"
    icon="mdi:console"
  />
</DocCardGrid>

## Next Steps

Once deployed, you'll want to populate your catalog with data assets:

<DocCardGrid>
  <DocCard
    title="Add Data with Plugins"
    description="Automatically discover assets from your data sources"
    href="/docs/Plugins"
    icon="mdi:puzzle"
  />
  <DocCard
    title="Configure Marmot"
    description="Customise authentication, settings, and more"
    href="/docs/Configure"
    icon="mdi:cog"
  />
</DocCardGrid>
