---
sidebar_position: 0
title: Docker Compose
---

# Docker Compose

Deploy Marmot with Docker Compose for production use.

import { CalloutCard, DocCard, DocCardGrid } from '@site/src/components/DocCard';

<CalloutCard
  title="Looking for a Quick Test?"
  description="Try the Quick Start guide for a simple local setup without encryption."
  href="/docs/quick-start"
  buttonText="Quick Start"
  icon="mdi:rocket-launch"
/>

## Prerequisites

- [Docker](https://docs.docker.com/get-docker/) and [Docker Compose](https://docs.docker.com/compose/install/) installed
- The Marmot CLI for generating encryption keys

Install the CLI:

```bash
curl -fsSL get.marmotdata.io | sh
```

## Generate an Encryption Key

Marmot encrypts sensitive credentials stored in your catalog. Generate a key before deploying:

```bash
marmot generate-encryption-key
```

Save this key somewhere safe.

## Create Your Docker Compose File

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
      MARMOT_DATABASE_PASSWORD: ${POSTGRES_PASSWORD}
      MARMOT_DATABASE_NAME: marmot
      MARMOT_DATABASE_SSLMODE: disable
      MARMOT_SERVER_ENCRYPTION_KEY: ${MARMOT_ENCRYPTION_KEY}
    depends_on:
      postgres:
        condition: service_healthy

  postgres:
    image: postgres:16-alpine
    environment:
      POSTGRES_USER: marmot
      POSTGRES_PASSWORD: ${POSTGRES_PASSWORD}
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

Create a `.env` file in the same directory:

```bash
POSTGRES_PASSWORD=your-secure-password
MARMOT_ENCRYPTION_KEY=your-generated-key
```

## Start Marmot

```bash
docker compose up -d
```

Open [http://localhost:8080](http://localhost:8080) and log in with `admin` / `admin`.

> Change the default password after your first login.

## Next Steps

<DocCardGrid>
  <DocCard
    title="Add Data with Plugins"
    description="Automatically discover assets from PostgreSQL, Kafka, S3 and more"
    href="/docs/Plugins"
    icon="mdi:puzzle"
  />
  <DocCard
    title="Configure Marmot"
    description="Set up authentication, SSO and more"
    href="/docs/Configure"
    icon="mdi:cog"
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
