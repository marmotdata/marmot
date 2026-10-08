# Deploy

There are multiple ways to deploy Marmot - choose whichever method works best with your existing infrastructure and workflows. If you would rather not run anything, [Marmot Cloud](https://cloud.marmotdata.io) is the managed edition and takes deployment off your plate entirely.

import { CalloutCard, DocCard, DocCardGrid } from '@site/src/components/DocCard';

<CalloutCard
  title="Skip the deployment entirely"
  description="Marmot Cloud provisions an instance with TLS, upgrades and daily backups in about a minute. Free account, billed only once you launch."
  href="https://cloud.marmotdata.io/signup"
  buttonText="Start free"
  icon="mdi:cloud-outline"
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
