---
sidebar_position: 0
title: Docker Compose
description: Run Marmot and PostgreSQL together with Docker Compose, with an encryption key for stored credentials.
---

# Docker Compose

Run Marmot and PostgreSQL together with Docker Compose. Unlike the [quick start](../quick-start.md), this setup encrypts the credentials Marmot stores.

import { Steps, Step } from '@site/src/components/Steps';

<Steps>
<Step title="Install the CLI">

```bash
curl -fsSL get.marmotdata.io | sh
```

</Step>
<Step title="Generate an encryption key">

Marmot encrypts the credentials it stores with this key. Keep it somewhere safe, because losing it means re-entering every stored credential:

```bash
marmot generate-encryption-key
```

</Step>
<Step title="Create your Compose file">

Save this as `docker-compose.yaml`:

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

</Step>
<Step title="Create your environment file">

Save this as `.env` in the same directory:

```bash
POSTGRES_PASSWORD=<database-password>
MARMOT_ENCRYPTION_KEY=<encryption-key>
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
</Steps>

## Next steps

- [Configuration reference](../Configure/index.md): every setting and its environment variable.
- [Plugins](../Plugins/index.md): catalog your first source.
- [Single sign-on](../Configure/Authentication/index.md): let your team sign in with your identity provider.
