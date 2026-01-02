---
sidebar_position: 2
---

# Quick Start

This guide will help you quickly spin up Marmot and populate it with sample assets and lineage relationships.

import { CalloutCard } from '@site/src/components/DocCard';

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

> **The default username and password is admin:admin**

<CalloutCard
  title="Need Help?"
  description="Join our Discord community to ask questions and connect with other Marmot users."
  href="https://discord.gg/tMgc9ayB"
  buttonText="Join Discord"
  variant="secondary"
  icon="mdi:account-group"
/>
