---
sidebar_position: 2
title: Quick Start
---

# Quick Start

Get Marmot running in seconds with Docker Compose.

import { CalloutCard, DocCard, DocCardGrid } from '@site/src/components/DocCard';
import { Steps, Step } from '@site/src/components/Steps';

<CalloutCard
  title="Just Want to Explore?"
  description="Try the live demo first to see Marmot's features without any setup."
  href="https://demo.marmotdata.io"
  buttonText="View Live Demo"
  icon="mdi:rocket-launch"
/>

<Steps>
  <Step title="Install the prerequisites">

[Docker](https://docs.docker.com/get-docker/) and [Docker Compose](https://docs.docker.com/compose/install/) installed.

  </Step>
  <Step title="Create your Docker Compose file">

Create a `docker-compose.yaml`:

```yaml
services:
  marmot:
    image: ghcr.io/marmotdata/marmot:latest
    ports:
      - "8080:8080"
    environment:
      MARMOT_DATABASE_HOST: postgres
      MARMOT_DATABASE_PORT: 5432
      MARMOT_DATABASE_USER: marmot
      MARMOT_DATABASE_PASSWORD: marmot
      MARMOT_DATABASE_NAME: marmot
      MARMOT_DATABASE_SSLMODE: disable
      MARMOT_SERVER_ALLOW_UNENCRYPTED: true
    depends_on:
      postgres:
        condition: service_healthy

  postgres:
    image: postgres:16-alpine
    environment:
      POSTGRES_USER: marmot
      POSTGRES_PASSWORD: marmot
      POSTGRES_DB: marmot
    volumes:
      - marmot_data:/var/lib/postgresql/data
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U marmot"]
      interval: 5s
      timeout: 5s
      retries: 5

volumes:
  marmot_data:
```

  </Step>
  <Step title="Start Marmot">

```bash
docker compose up -d
```

Open [http://localhost:8080](http://localhost:8080) and log in with `admin` / `admin`.

  </Step>
</Steps>

## Next Steps

<DocCardGrid>
  <DocCard
    title="Add Data with Plugins"
    description="Automatically discover assets from PostgreSQL, Kafka, S3 and more"
    docId="Plugins/index"
    icon="mdi:puzzle"
  />
  <DocCard
    title="Configure Marmot"
    description="Set up authentication, customise settings and more"
    docId="Configure/index"
    icon="mdi:cog"
  />
  <DocCard
    title="Learn the Query Language"
    description="Find any asset with powerful search queries"
    docId="queries"
    icon="mdi:magnify"
  />
  <DocCard
    title="Production Deployment"
    description="Deploy with Docker Compose, Kubernetes or the CLI"
    docId="Deploy/index"
    icon="mdi:cloud-upload"
  />
</DocCardGrid>

<CalloutCard
  title="Need Help?"
  description="Join the Discord community to get support and connect with other Marmot users."
  href="https://discord.gg/TWCk7hVFN4"
  buttonText="Join Discord"
  variant="secondary"
  icon="mdi:account-group"
/>
