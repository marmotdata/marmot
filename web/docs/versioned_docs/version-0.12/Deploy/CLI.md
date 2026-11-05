---
sidebar_position: 4
title: Binary
description: Run the Marmot server from the single binary against your own PostgreSQL.
---

# Binary

Run Marmot directly on a server from the single binary, against a PostgreSQL database you already have.

import { Steps, Step, TipBox } from '@site/src/components/Steps';
import { CliInstall } from '@site/src/components/CliInstall';

## Install

<CliInstall />

## Start the server

<Steps>
<Step title="Generate an encryption key">

Marmot encrypts the credentials it stores with this key. Keep it somewhere safe, because losing it means re-entering every stored credential:

```bash
marmot generate-encryption-key
```

</Step>
<Step title="Create a config file">

Save your database settings as `config.yaml`:

```yaml
database:
  host: localhost
  port: 5432
  user: marmot
  password: "<database-password>"
  name: marmot
```

</Step>
<Step title="Start the server">

`marmot run` reads `config.yaml` from the current directory, or the file you pass with `--config`:

```bash
export MARMOT_SERVER_ENCRYPTION_KEY="<encryption-key>"
marmot run --config config.yaml
```

</Step>
<Step title="Sign in">

Open [http://localhost:8080](http://localhost:8080) and sign in as `admin` with the password `admin`. Marmot asks you to choose a new password straight away.

</Step>
</Steps>

## Without an encryption key

For local development only, Marmot can run without a key and store credentials unencrypted:

```yaml
server:
  allow_unencrypted: true

database:
  host: localhost
  port: 5432
  user: marmot
  password: "<database-password>"
  name: marmot
```

<TipBox variant="warning" title="Development only">
Never set `allow_unencrypted: true` in production. Every stored credential would be readable by anyone with access to the database.
</TipBox>

## Next steps

- [Configuration reference](../Configure/index.md): every setting and its environment variable.
- [Plugins](../Plugins/index.md): catalog your first source.
- [Single sign-on](../Configure/Authentication/index.md): let your team sign in with your identity provider.
