<div align="center">
  <img src="https://marmotdata.github.io/charts/marmot.svg" width="120">

# Marmot Helm Chart

Deploy Marmot to your Kubernetes cluster using the official Helm chart.

[Documentation](https://marmotdata.io/docs/Deploy/Helm) | [Live Demo](https://demo.marmotdata.io) | [GitHub](https://github.com/marmotdata/marmot)

</div>

## What is Marmot?

Marmot is an open-source data catalog that helps teams discover, understand, and govern their data assets. It's designed for modern data ecosystems where data flows through multiple systems, formats, and teams.

## Quick Start

### Add the Helm repository

```bash
helm repo add marmotdata https://marmotdata.github.io/charts
helm repo update
```

### Install Marmot

```bash
helm install marmot marmotdata/marmot
```

### Access the UI

Port-forward to access the dashboard:

```bash
kubectl port-forward svc/marmot 8080:8080
```

Open [http://localhost:8080](http://localhost:8080) in your browser.

> The default username and password is **admin:admin**. Change this after your first login.

## Database Configuration

Marmot requires PostgreSQL. Choose one of the following options:

### External PostgreSQL

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

### Embedded PostgreSQL

For development and testing, enable the embedded PostgreSQL:

```bash
helm install marmot marmotdata/marmot \
  --set postgresql.enabled=true
```

### CloudNativePG

For production Kubernetes deployments, [CloudNativePG](https://cloudnative-pg.io/) provides a robust PostgreSQL operator with automatic failover, read replicas and connection pooling.

1. Install the CloudNativePG operator:

```bash
kubectl apply --server-side -f \
  https://raw.githubusercontent.com/cloudnative-pg/cloudnative-pg/main/releases/cnpg-1.25.1.yaml
```

2. Configure and install Marmot with CNPG:

```yaml
# values.yaml
cnpg:
  enabled: true
  instances: 3
  password: "your-secure-password"

  parameters:
    shared_buffers: "256MB"
    work_mem: "64MB"
    effective_cache_size: "1GB"

  persistence:
    size: "10Gi"

  pooler:
    enabled: true
    instances: 2
    poolMode: "transaction"
```

```bash
helm install marmot marmotdata/marmot -f values.yaml
```

3. Verify the cluster is ready:

```bash
kubectl get clusters
kubectl get pods -l cnpg.io/cluster=marmot-cnpg
```

All pods should show `Running` status with the primary indicated by the `-1` suffix.

## Encryption Key

Marmot encrypts sensitive pipeline credentials. You must configure an encryption key.

### Auto-Generated (Default)

The Helm chart can auto-generate an encryption key for you (enabled by default):

```yaml
config:
  server:
    autoGenerateEncryptionKey: true
```

> **Back Up Your Key**: If the generated secret is deleted, you'll lose access to encrypted credentials. Back it up immediately after installation.

Retrieve the auto-generated key:

```bash
kubectl get secret <release-name>-marmot-encryption-key \
  -o jsonpath='{.data.encryption-key}' | base64 -d
```

### Bring Your Own

1. Generate an encryption key:

```bash
marmot generate-encryption-key
# or
openssl rand -base64 32
```

2. Create a Kubernetes secret:

```bash
kubectl create secret generic marmot-encryption \
  --from-literal=encryption-key="your-generated-key"
```

3. Configure the Helm chart:

```yaml
config:
  server:
    autoGenerateEncryptionKey: false
    encryptionKeySecretRef:
      name: marmot-encryption
      key: encryption-key
```

## Reference

For all available configuration options, view the chart's defaults:

```bash
helm show values marmotdata/marmot
```

Or browse the [values.yaml on GitHub](https://github.com/marmotdata/marmot/blob/main/charts/marmot/values.yaml).

## Links

- [Full Documentation](https://marmotdata.io/docs/introduction)
- [Helm / Kubernetes Guide](https://marmotdata.io/docs/Deploy/Helm)
- [GitHub](https://github.com/marmotdata/marmot)
- [Discord Community](https://discord.gg/TWCk7hVFN4)
