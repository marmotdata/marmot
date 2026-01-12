---
sidebar_position: 2
title: Docker
---

# Docker

Deploy Marmot using Docker containers - recommended for most users.

## Generate Encryption Key

Install Marmot CLI and generate a key:

```bash
curl -fsSL get.marmotdata.io | sh
marmot generate-encryption-key
```

## Simple Deployment

Run Marmot with environment variables:

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

> **The default username and password is admin:admin**

## Configuration

Configure via mounted config file:

```bash
docker run -d \
  --name marmot \
  -p 8080:8080 \
  -v /path/to/config.yaml:/app/config.yaml \
  ghcr.io/marmotdata/marmot:latest server --config /app/config.yaml
```

### Development Without Encryption

For development only (credentials stored in plaintext):

```bash
docker run -d \
  --name marmot \
  -p 8080:8080 \
  -e MARMOT_SERVER_ALLOW_UNENCRYPTED=true \
  -e MARMOT_DATABASE_HOST=localhost \
  ghcr.io/marmotdata/marmot:latest
```

For all configuration options, see [available configuration here.](/docs/configure)
