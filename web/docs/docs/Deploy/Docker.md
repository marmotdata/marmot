---
sidebar_position: 2
title: Docker
description: Run the Marmot container against your own PostgreSQL database.
---

# Docker

Run the Marmot container against a PostgreSQL database you already have.

import { Steps, Step, Tabs, TabPanel, TipBox } from '@site/src/components/Steps';

<Steps>
<Step title="Generate an encryption key">

Install the Marmot CLI and generate a key. Keep it somewhere safe, because losing it means re-entering every stored credential:

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
  -e MARMOT_SERVER_ENCRYPTION_KEY=<encryption-key> \
  -e MARMOT_DATABASE_HOST=<database-host> \
  -e MARMOT_DATABASE_PORT=5432 \
  -e MARMOT_DATABASE_USER=<database-user> \
  -e MARMOT_DATABASE_PASSWORD=<database-password> \
  -e MARMOT_DATABASE_NAME=marmot \
  -e MARMOT_DATABASE_SSLMODE=disable \
  ghcr.io/marmotdata/marmot:latest
```

</Step>
<Step title="Sign in">

Open [http://localhost:8080](http://localhost:8080) and sign in as `admin` with the password `admin`. Marmot asks you to choose a new password straight away.

</Step>
</Steps>

## Configuration

<Tabs items={[
{ label: "Environment variables", value: "env" },
{ label: "Config file", value: "file" }
]}>
<TabPanel>

Pass every setting as an environment variable:

```bash
docker run -d \
  --name marmot \
  -p 8080:8080 \
  -e MARMOT_SERVER_ENCRYPTION_KEY=<encryption-key> \
  -e MARMOT_DATABASE_HOST=db.internal \
  -e MARMOT_DATABASE_PORT=5432 \
  -e MARMOT_DATABASE_USER=marmot \
  -e MARMOT_DATABASE_PASSWORD=<database-password> \
  -e MARMOT_DATABASE_NAME=marmot \
  -e MARMOT_DATABASE_SSLMODE=require \
  ghcr.io/marmotdata/marmot:latest
```

</TabPanel>
<TabPanel>

Or mount a config file:

```bash
docker run -d \
  --name marmot \
  -p 8080:8080 \
  -v /path/to/config.yaml:/app/config.yaml \
  ghcr.io/marmotdata/marmot:latest run --config /app/config.yaml
```

With a `config.yaml` like this:

```yaml
server:
  encryption_key: "<encryption-key>"

database:
  host: db.internal
  port: 5432
  user: marmot
  password: "<database-password>"
  name: marmot
  sslmode: require
```

</TabPanel>
</Tabs>

## Without an encryption key

For local development only, Marmot can run without a key and store credentials unencrypted:

```bash
docker run -d \
  --name marmot \
  -p 8080:8080 \
  -e MARMOT_SERVER_ALLOW_UNENCRYPTED=true \
  -e MARMOT_DATABASE_HOST=host.docker.internal \
  -e MARMOT_DATABASE_PORT=5432 \
  -e MARMOT_DATABASE_USER=postgres \
  -e MARMOT_DATABASE_PASSWORD=<database-password> \
  -e MARMOT_DATABASE_NAME=marmot \
  ghcr.io/marmotdata/marmot:latest
```

<TipBox variant="warning" title="Development only">
Never set `MARMOT_SERVER_ALLOW_UNENCRYPTED=true` in production. Every stored credential would be readable by anyone with access to the database.
</TipBox>

## Next steps

- [Configuration reference](../Configure/index.md): every setting and its environment variable.
- [Plugins](../Plugins/index.md): catalog your first source.
- [Single sign-on](../Configure/Authentication/index.md): let your team sign in with your identity provider.
