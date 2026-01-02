---
sidebar_position: 2
title: Helm / Kubernetes
---

# Helm / Kubernetes

Deploy Marmot to your Kubernetes cluster using our official Helm chart.

## Simple Deployment

Add the Marmot Helm repository and install:

```bash
helm repo add marmotdata https://marmotdata.github.io/charts
helm repo update
helm install marmot marmotdata/marmot
```

> **The default username and password is admin:admin**

## With External PostgreSQL

Deploy Marmot with your existing PostgreSQL database:

```bash
helm install marmot marmotdata/marmot \
  --set config.database.host=your-postgres-host \
  --set config.database.user=your-postgres-user \
  --set config.database.password=your-postgres-password \
  --set config.database.name=your-postgres-database
```

## With Embedded PostgreSQL

For testing or development, you can enable the embedded PostgreSQL:

```bash
helm install marmot marmotdata/marmot \
  --set postgresql.enabled=true
```

> ⚠️ **The embedded PostgreSQL is NOT recommended for production use.**

## Configuration

You can configure Marmot using a custom values file:

```yaml
# custom-values.yaml
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

ingress:
  enabled: true
  className: nginx
  hosts:
    - host: marmot.example.com
      paths:
        - path: /
          pathType: Prefix

resources:
  limits:
    cpu: 1000m
    memory: 1Gi
  requests:
    cpu: 200m
    memory: 256Mi
```

Deploy with your custom configuration:

```bash
helm install marmot marmotdata/marmot -f custom-values.yaml
```

## Encryption Key

Marmot requires an encryption key to protect sensitive pipeline credentials. The Helm chart auto-generates one by default:

```yaml
config:
  server:
    autoGenerateEncryptionKey: true
```

**Back up the auto-generated key:**

```bash
kubectl get secret <release-name>-marmot-encryption-key \
  -o jsonpath='{.data.encryption-key}' | base64 -d
```

### Use Your Own Key

Generate a key:

```bash
marmot generate-encryption-key
# or
openssl rand -base64 32
```

Create a secret:

```bash
kubectl create secret generic marmot-encryption \
  --from-literal=encryption-key="your-generated-key"
```

Configure Marmot to use it:

```yaml
config:
  server:
    autoGenerateEncryptionKey: false
    encryptionKeySecretRef:
      name: marmot-encryption
      key: encryption-key
```

### Disable Encryption (Not Recommended)

For development/testing only:

```yaml
config:
  server:
    autoGenerateEncryptionKey: false
    allowUnencrypted: true
```

For all available configuration options, see the [chart's values.yaml](https://github.com/marmotdata/marmot/blob/main/charts/marmot/values.yaml) or run:

```bash
helm show values marmotdata/marmot
```
