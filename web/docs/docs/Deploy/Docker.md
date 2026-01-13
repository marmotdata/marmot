---
sidebar_position: 2
title: Docker
---

# Docker

Deploy Marmot using Docker containers with your own PostgreSQL database.

import { DocCard, DocCardGrid } from '@site/src/components/DocCard';
import { Steps, Step, Tabs, TabPanel, TipBox } from '@site/src/components/Steps';

## Quick Start

<Steps>
<Step title="Generate an encryption key">

Install the Marmot CLI and generate a key:

```bash
curl -fsSL get.marmotdata.io | sh
marmot generate-encryption-key
```

</Step>
<Step title="Run the container">

```bash
docker run -d \
  --name marmot \
  -p 8080:8080 \
  -e MARMOT_SERVER_ENCRYPTION_KEY=<your-generated-key> \
  -e MARMOT_DATABASE_HOST=<your-postgres-host> \
  -e MARMOT_DATABASE_PORT=5432 \
  -e MARMOT_DATABASE_USER=<your-postgres-user> \
  -e MARMOT_DATABASE_PASSWORD=<your-postgres-password> \
  -e MARMOT_DATABASE_NAME=<your-postgres-database> \
  -e MARMOT_DATABASE_SSLMODE=disable \
  ghcr.io/marmotdata/marmot:latest
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

## Configuration

<Tabs items={[
{ label: "Environment Variables", value: "env", icon: "mdi:application-variable" },
{ label: "Config File", value: "file", icon: "mdi:file-cog" }
]}>
<TabPanel>

Pass configuration as environment variables:

```bash
docker run -d \
  --name marmot \
  -p 8080:8080 \
  -e MARMOT_SERVER_ENCRYPTION_KEY=your-key \
  -e MARMOT_DATABASE_HOST=postgres.example.com \
  -e MARMOT_DATABASE_PORT=5432 \
  -e MARMOT_DATABASE_USER=marmot \
  -e MARMOT_DATABASE_PASSWORD=secret \
  -e MARMOT_DATABASE_NAME=marmot \
  -e MARMOT_DATABASE_SSLMODE=require \
  ghcr.io/marmotdata/marmot:latest
```

</TabPanel>
<TabPanel>

Mount a config file for more complex configurations:

```bash
docker run -d \
  --name marmot \
  -p 8080:8080 \
  -v /path/to/config.yaml:/app/config.yaml \
  ghcr.io/marmotdata/marmot:latest server --config /app/config.yaml
```

Example `config.yaml`:

```yaml
server:
  encryption_key: "your-generated-key"

database:
  host: postgres.example.com
  port: 5432
  user: marmot
  password: secret
  name: marmot
  sslmode: require
```

</TabPanel>
</Tabs>

---

## Development Mode

For local development, you can skip encryption (credentials stored in plaintext):

```bash
docker run -d \
  --name marmot \
  -p 8080:8080 \
  -e MARMOT_SERVER_ALLOW_UNENCRYPTED=true \
  -e MARMOT_DATABASE_HOST=host.docker.internal \
  -e MARMOT_DATABASE_PORT=5432 \
  -e MARMOT_DATABASE_USER=postgres \
  -e MARMOT_DATABASE_PASSWORD=password \
  -e MARMOT_DATABASE_NAME=marmot \
  ghcr.io/marmotdata/marmot:latest
```

<TipBox variant="warning" title="Not for Production">
Never use `MARMOT_SERVER_ALLOW_UNENCRYPTED=true` in production environments.
</TipBox>

---

## Reference

For all configuration options, see the [configuration guide](/docs/Configure).

## Next Steps

<DocCardGrid>
<DocCard
  title="Add Data with Plugins"
  description="Automatically discover assets from your data sources"
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
