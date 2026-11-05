---
sidebar_position: 6
title: API
---

# API

The REST API creates, updates and deletes assets, lineage and metadata from your own tooling. The [API reference](/api) documents every endpoint, and [lineage](../lineage.md#send-lineage-from-your-own-pipelines) shows how to send edges with column mappings.

Send an API key with every request:

```text
X-API-Key: <api-key>
```

Create a key under **Account → API keys**, or use a [service account](../Configure/access-control.md) for anything that isn't a person.
