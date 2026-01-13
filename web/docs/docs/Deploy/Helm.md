---
sidebar_position: 3
title: Helm / Kubernetes
---

# Helm / Kubernetes

Deploy Marmot to your Kubernetes cluster using our official Helm chart.

import { DocCard, DocCardGrid } from '@site/src/components/DocCard';
import { Steps, Step, Tabs, TabPanel, TipBox } from '@site/src/components/Steps';

## Quick Start

<Steps>
  <Step title="Add the Helm repository">

```bash
helm repo add marmotdata https://marmotdata.github.io/charts
helm repo update
```

  </Step>
  <Step title="Install Marmot">

```bash
helm install marmot marmotdata/marmot
```

  </Step>
  <Step title="Access the UI">

Port-forward to access the dashboard:

```bash
kubectl port-forward svc/marmot 8080:8080
```

Open [http://localhost:8080](http://localhost:8080) in your browser.

  </Step>
</Steps>

<TipBox variant="info" title="Default Credentials">
The default username and password is **admin:admin**. Change this after your first login.
</TipBox>

---

## Database Configuration

Marmot requires PostgreSQL. Choose one of the following options:

<Tabs items={[
{ label: "External PostgreSQL", value: "external", icon: "mdi:database" },
{ label: "Embedded PostgreSQL", value: "embedded", icon: "mdi:database-cog" }
]}>
<TabPanel>

Connect Marmot to your existing PostgreSQL database:

```yaml
config:
  database:
    host: postgres.example.com
    port: 5432
    user: marmot
    passwordSecretRef:
      name: marmot-db-secret
      key: password
    name: marmot
    sslmode: require
```

  </TabPanel>
  <TabPanel>

For development and testing, enable the embedded PostgreSQL:

```bash
helm install marmot marmotdata/marmot \
  --set postgresql.enabled=true
```

  </TabPanel>
</Tabs>

---

## Encryption Key

Marmot encrypts sensitive pipeline credentials at rest. You must configure an encryption key.

<Tabs items={[
{ label: "Auto-Generated", value: "auto", icon: "mdi:key-plus" },
{ label: "Bring Your Own", value: "custom", icon: "mdi:key" },
{ label: "Disable (Dev Only)", value: "disable", icon: "mdi:key-remove" }
]}>
<TabPanel>

The Helm chart can auto-generate an encryption key for you (enabled by default):

```yaml
config:
  server:
    autoGenerateEncryptionKey: true
```

<TipBox variant="warning" title="Back Up Your Key">
If the generated secret is deleted, you'll lose access to encrypted credentials. Back it up immediately after installation.
</TipBox>

Retrieve the auto-generated key:

```bash
kubectl get secret <release-name>-marmot-encryption-key \
  -o jsonpath='{.data.encryption-key}' | base64 -d
```

  </TabPanel>
  <TabPanel>

<Steps>
  <Step title="Generate an encryption key">

```bash
marmot generate-encryption-key
# or
openssl rand -base64 32
```

  </Step>
  <Step title="Create a Kubernetes secret">

```bash
kubectl create secret generic marmot-encryption \
  --from-literal=encryption-key="your-generated-key"
```

  </Step>
  <Step title="Configure the Helm chart">

```yaml
config:
  server:
    autoGenerateEncryptionKey: false
    encryptionKeySecretRef:
      name: marmot-encryption
      key: encryption-key
```

  </Step>
</Steps>

  </TabPanel>
  <TabPanel>

For local development only, you can disable encryption entirely:

```yaml
config:
  server:
    autoGenerateEncryptionKey: false
    allowUnencrypted: true
```

<TipBox variant="danger" title="Security Risk">
This stores all credentials in plaintext. Never use this in production.
</TipBox>

  </TabPanel>
</Tabs>

---

## Reference

For all available configuration options, view the chart's defaults:

```bash
helm show values marmotdata/marmot
```

Or browse the [values.yaml on GitHub](https://github.com/marmotdata/marmot/blob/main/charts/marmot/values.yaml).

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
