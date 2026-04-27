---
sidebar_position: 4
title: CLI / Binary
---

# CLI / Binary

Run Marmot directly on your system using the single binary.

import { DocCard, DocCardGrid } from '@site/src/components/DocCard';
import { Steps, Step, Tabs, TabPanel, TipBox } from '@site/src/components/Steps';

## Installation

<Tabs items={[
{ label: "Automatic", value: "auto", icon: "mdi:download" },
{ label: "Manual", value: "manual", icon: "mdi:folder-download" }
]}>
<TabPanel>

Install Marmot with the installation script:

```bash
curl -fsSL get.marmotdata.io | sh
```

<TipBox variant="info" title="Verify Scripts">
It's good practice to inspect the contents of any script before piping it into bash.
</TipBox>

</TabPanel>
<TabPanel>

<Steps>
<Step title="Download the binary">

Download the latest Marmot binary for your platform from [GitHub Releases](https://github.com/marmotdata/marmot/releases).

</Step>
<Step title="Make it executable">

```bash
chmod +x marmot
```

</Step>
<Step title="Move to your PATH">

```bash
sudo mv marmot /usr/local/bin/
```

</Step>
</Steps>

</TabPanel>
</Tabs>

---

## Quick Start

<Steps>
<Step title="Generate an encryption key">

```bash
marmot generate-encryption-key
```

Save this key securely. You'll need it to start the server.

</Step>
<Step title="Create a config file">

Create `config.yaml` with your database settings:

```yaml
database:
  host: localhost
  port: 5432
  user: postgres
  password: your-password
  name: marmot
```

</Step>
<Step title="Start the server">

```bash
export MARMOT_SERVER_ENCRYPTION_KEY="your-generated-key"
marmot server --config config.yaml
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

## Development Mode

For local development, you can skip encryption (credentials stored in plaintext):

```yaml
server:
  allow_unencrypted: true

database:
  host: localhost
  port: 5432
  user: postgres
  password: password
  name: marmot
```

```bash
marmot server --config config.yaml
```

<TipBox variant="warning" title="Not for Production">
Never use `allow_unencrypted: true` in production environments.
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
