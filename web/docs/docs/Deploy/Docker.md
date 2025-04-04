---
sidebar_position: 1
---

This guide covers deploying Marmot using Docker.

## Simple Deployment

Run Marmot with environment variables for configuration:

```bash
docker run -d \
  --name marmot \
  -p 8080:8080 \
  -e DATABASE_HOST=<your-postgres-host> \
  -e DATABASE_PORT=5432 \
  -e DATABASE_USER=<your-postgres-user> \
  -e DATABASE_PASSWORD=<your-postgres-password> \
  -e DATABASE_NAME=<your-postgres-database> \
  -e DATABASE_SSLMODE=disable \
  ghcr.io/marmotdata/marmot:0.1.0
```

## Configuration

You can configure Marmot using environment variables or by mounting a configuration file. You can read more about [available configuration options here.](/docs/configure)

```bash
docker run -d \
  --name marmot \
  -p 8080:8080 \
  -v /path/to/config.yaml:/app/config.yaml \
  ghcr.io/marmotdata/marmot:0.1.0 run --config /app/config.yaml
```
