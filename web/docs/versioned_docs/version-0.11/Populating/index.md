# Populating your catalog

There are two ways to fill the catalog. A pipeline points a plugin at a source and discovers what is there on a schedule. Declaring assets as code describes them yourself, next to the infrastructure that creates them. Most catalogs use both: pipelines for the bulk of the catalog, code for the services and APIs no plugin can see.

import { CalloutCard, DocCard, DocCardGrid } from '@site/src/components/DocCard';

<CalloutCard
  title="Start with a pipeline"
  description="Point a plugin at a database, warehouse or broker and let it discover your assets."
  docId="Populating/Pipelines"
  buttonText="Pipelines"
  icon="mdi:pipe"
/>

## Discover with plugins

<DocCardGrid>
  <DocCard
    title="Pipelines"
    description="The server runs a plugin on a schedule. Declare it in Terraform or create it in the UI."
    docId="Populating/Pipelines"
    icon="mdi:pipe"
  />
  <DocCard
    title="UI"
    description="Create a pipeline from a form on the Runs page"
    docId="Populating/UI"
    icon="mdi:cursor-default-click"
  />
  <DocCard
    title="CLI"
    description="Run plugins from any machine or CI job and push the results"
    docId="Populating/CLI"
    icon="mdi:console"
  />
  <DocCard
    title="Kubernetes operator"
    description="Run each pipeline as its own Job or CronJob in your cluster"
    docId="Populating/Operator"
    icon="mdi:kubernetes"
  />
</DocCardGrid>

## Declare assets as code

<DocCardGrid>
  <DocCard
    title="Terraform"
    description="Assets, lineage, glossary terms and data products as Terraform resources"
    docId="Populating/Terraform"
    icon="mdi:terraform"
  />
  <DocCard
    title="Pulumi"
    description="The same resources in the language you already use"
    docId="Populating/Pulumi"
    icon="mdi:code-braces"
  />
  <DocCard
    title="REST API"
    description="Create and update assets from your own tooling"
    docId="Populating/API"
    icon="mdi:api"
  />
</DocCardGrid>

<CalloutCard
  title="Browse every plugin"
  description="Configuration options, required permissions and examples for every source Marmot can catalog."
  href="https://plugins.marmotdata.io"
  buttonText="Plugin registry"
  variant="external"
  icon="mdi:puzzle"
/>
