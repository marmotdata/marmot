---
sidebar_position: 2
---

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

For all available configuration options, see the [chart's values.yaml](https://github.com/marmotdata/marmot/blob/main/charts/marmot/values.yaml) or run:

```bash
helm show values marmotdata/marmot
```
