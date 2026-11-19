---
title: Self-host
description: Run Marmot on your own infrastructure with Docker Compose, Docker, Helm or the binary.
---

# Self-host

Marmot is one Go binary backed by PostgreSQL, so it runs wherever you already run containers or services. Pick the method that matches your infrastructure, then use the configuration pages that follow to set up TLS, search and monitoring. To skip running it yourself, [Marmot Cloud](../Cloud/index.md) is the managed edition.

import { DocCard, DocCardGrid } from '@site/src/components/DocCard';

<DocCardGrid>
  <DocCard
    title="Docker Compose"
    description="Marmot and PostgreSQL together with one command"
    docId="Deploy/Docker-Compose"
    icon="mdi:docker"
  />
  <DocCard
    title="Docker"
    description="The Marmot container with your own PostgreSQL"
    docId="Deploy/Docker"
    icon="mdi:docker"
  />
  <DocCard
    title="Helm and Kubernetes"
    description="The official Helm chart for Kubernetes clusters"
    docId="Deploy/Helm"
    icon="mdi:kubernetes"
  />
  <DocCard
    title="Binary"
    description="The single binary, run directly on a server"
    docId="Deploy/CLI"
    icon="mdi:console"
  />
  <DocCard
    title="Configuration reference"
    description="Every setting, its environment variable and its default"
    docId="Configure/index"
    icon="mdi:cog"
  />
</DocCardGrid>
