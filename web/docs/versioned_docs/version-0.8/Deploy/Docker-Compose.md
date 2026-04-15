---
sidebar_position: 0
title: Docker Compose
---

# Docker Compose

Deploy Marmot and PostgreSQL together with Docker Compose.

import { CalloutCard, DocCard, DocCardGrid } from '@site/src/components/DocCard';
import { Steps, Step, TipBox } from '@site/src/components/Steps';

<CalloutCard
  title="Looking for a Quick Test?"
  description="Try the Quick Start guide for a simple local setup without encryption."
  href="/docs/quick-start"
  buttonText="Quick Start"
  icon="mdi:rocket-launch"
/>

## Quick Start

<Steps>
<Step title="Install the CLI">

```bash
curl -fsSL get.marmotdata.io | sh
```

</Step>
<Step title="Generate an encryption key">

Marmot encrypts sensitive credentials stored in your catalog:

```bash
marmot generate-encryption-key
```

Save this key securely. You'll need it in the next step.

</Step>
<Step title="Create your compose file">

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

</Step>
<Step title="Create your environment file">

Create a `.env` file in the same directory:

```bash
POSTGRES_PASSWORD=your-secure-password
MARMOT_ENCRYPTION_KEY=your-generated-key
```

</Step>
<Step title="Start Marmot">

```bash
docker compose up -d
```

</Step>
<Step title="Access the UI">

Open [http://localhost:8080](http://localhost:8080) in your browser.

</Step>
</Steps>

<TipBox variant="info" title="Default Credentials">
The default username and password is **admin:admin**. Change this after your first login.
</TipBox>

---

## Reference

For all configuration options, see the [configuration guide](/docs/Configure).

## Next Steps

<DocCardGrid>
<DocCard
  title="Add Data with Plugins"
  description="Automatically discover assets from PostgreSQL, Kafka, S3 and more"
  href="/docs/Plugins"
  icon="mdi:puzzle"
/>
<DocCard
  title="Configure Authentication"
  description="Set up SSO with GitHub, Google, Okta and more"
  href="/docs/Configure/Authentication"
  icon="mdi:shield-account"
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
