---
sidebar_position: 2
title: Quick start
description: Run Marmot on your machine with Docker Compose and catalog your first source.
---

import { Steps, Step } from '@site/src/components/Steps';
import { ThemedImg } from '@site/src/components/ThemedImg';

# Quick start

By the end of this page you'll have Marmot running on your machine with your first source cataloged and searchable in **Discover**. To skip running it yourself, [Marmot Cloud](Cloud/getting-started.md) launches a managed instance instead.

## Before you start

- [Docker](https://docs.docker.com/get-docker/) and [Docker Compose](https://docs.docker.com/compose/install/) are installed.
- Port 8080 is free.

<Steps>
  <Step title="Create the Compose file">

Save this as `docker-compose.yaml`. It runs Marmot and PostgreSQL for trying things out, with credentials stored unencrypted:

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

  </Step>
  <Step title="Sign in">

Open [http://localhost:8080](http://localhost:8080) and sign in as `admin` with the password `admin`. Marmot asks you to choose a new password straight away.

  </Step>
  <Step title="Catalog your first source">

In Marmot, open **Plugins** and pick your system under **Add a source**:

<ThemedImg
  lightSrc="/img/quickstart-add-source-light.png"
  darkSrc="/img/quickstart-add-source-dark.png"
  alt="The Add a source section of the Plugins page, with plugins grouped into databases, transformation, data integration and catalogs"
/>

1. Enter a **Pipeline name**, such as `warehouse`, and select **Continue**.
2. Fill in the connection details and select **Check and continue**.
3. Select **Create pipeline**.
4. On the pipeline's page, select **Run now**.

  </Step>
  <Step title="Find it in Discover">

When the run finishes, open **Discover**. Your source's tables, topics or files are listed there, and you can search them alongside glossary terms, data products and teams:

<ThemedImg
  lightSrc="/img/quickstart-discover-light.png"
  darkSrc="/img/quickstart-discover-dark.png"
  alt="Discover listing cataloged assets with filters for kind, type and provider"
  size="window"
/>

  </Step>
</Steps>

## Next steps

- [Self-host](Deploy/index.md): run Marmot for your team, with an encryption key for stored credentials.
- [Plugins](Plugins/index.md): every system Marmot can catalog.
- [Search and the query language](queries.md): find assets precisely.
- Ask for help in the [Marmot Discord](https://discord.gg/TWCk7hVFN4).
