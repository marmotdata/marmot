# Deploy

There are multiple ways to deploy Marmot - choose whichever method works best with your existing infrastructure and workflows.

import { CalloutCard, DocCard, DocCardGrid } from '@site/src/components/DocCard';

<CalloutCard
  title="Get Started in Five Minutes"
  description="Follow the Quick Start Guide to try Marmot out locally."
  docId="quick-start"
  buttonText="Quick Start"
  icon="mdi:rocket-launch"
/>

<CalloutCard
  title="Would rather not run it?"
  description="Marmot Cloud gives you a managed instance on its own hostname, with the database, upgrades, backups and TLS handled for you."
  docId="Cloud/index"
  buttonText="Marmot Cloud"
  variant="secondary"
  icon="mdi:cloud-outline"
/>

## Deployment Options

<DocCardGrid>
  <DocCard
    title="Docker Compose"
    description="Deploy Marmot and PostgreSQL together with one command"
    docId="Deploy/Docker-Compose"
    icon="mdi:docker"
  />
  <DocCard
    title="Docker"
    description="Deploy using containers with your own PostgreSQL"
    docId="Deploy/Docker"
    icon="mdi:docker"
  />
  <DocCard
    title="Helm / Kubernetes"
    description="Deploy to Kubernetes clusters with the official Helm chart"
    docId="Deploy/Helm"
    icon="mdi:kubernetes"
  />
  <DocCard
    title="CLI / Binary"
    description="Run directly on your system with the single binary"
    docId="Deploy/CLI"
    icon="mdi:console"
  />
</DocCardGrid>

## Next Steps

Once deployed, you'll want to populate your catalog with data assets:

<DocCardGrid>
  <DocCard
    title="Add Data with Plugins"
    description="Automatically discover assets from your data sources"
    docId="Plugins/index"
    icon="mdi:puzzle"
  />
  <DocCard
    title="Configure Marmot"
    description="Customise authentication, settings, and more"
    docId="Configure/index"
    icon="mdi:cog"
  />
</DocCardGrid>
