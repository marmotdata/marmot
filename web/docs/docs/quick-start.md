---
sidebar_position: 2
---

# Quick Start

This guide will help you quickly spin up Marmot and populate it with sample assets and lineage relationships.

## Requirements

- Docker and Docker Compose
- git

## Getting Started

1. Clone the repository:

```bash
git clone https://github.com/marmotdata/marmot
```

2. Navigate to the quickstart directory

```bash
cd marmot/examples/quickstart
```

3. Start the example

```bash
docker compose up
```

4. Once started, you should be able to acces to the Marmot UI at [http://localhost:8080](http://localhost:8080)

## What's created?

The quick start automatically creates a few assets with schemas and lineage to showcase its functionality. The following assets are created with lineage connecting them:

- Kafka Topic with a Schema
- PostgreSQL Database with a Schema
- S3 Bucket
- Kubernetes Service

You can start by looking at the Kafka Topic that is created at [http://localhost:8080/assets/topic/customer-events-stream](http://localhost:8080/assets/topic/customer-events-stream), you can see it has some metadata assigned to it, if you go to the `Environments` tab, you can see some environment level variations of this metadata. The `Schema` tab will show you a nicely formatted schema and some examples along with a schema for the headers. The `Lineage` tab showcases the flow of data through all the assets.

## Next steps

After exploring the pre-populated assets, you can start looking at ingesting your real production data. There are many ways of ingesting assets, you can read more about [populating the catalog here](docs/populating)
