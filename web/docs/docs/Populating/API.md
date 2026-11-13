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

## Create or update an asset by MRN

`PUT /api/v1/assets/by-mrn` creates the asset an [MRN](../MRNs/index.md) names, or updates it when it exists. It's the call to use from automation that runs more than once:

```bash
curl -X PUT https://marmot.example.com/api/v1/assets/by-mrn \
  -H "X-API-Key: <api-key>" \
  -H "Content-Type: application/json" \
  -d '{
    "mrn": "mrn://asset/table/postgresql@prod-eu/analytics.public.orders",
    "description": "One row per order, from payment authorisation."
  }'
```

What you send is kept as your own contribution, so the next pipeline run doesn't overwrite it. Leave out `name`, `type` and `providers` and Marmot takes them from the MRN.

`POST /api/v1/assets/` creates an asset and fails if one already has its MRN. Set `mrn` to choose the MRN, or `zone` to have Marmot build it from the type, first provider and name. A taken MRN returns `409 Conflict` with the asset that holds it:

```json
{
  "error": "An asset with MRN mrn://asset/table/postgresql/orders already exists. Import or update it, or set mrn or zone to tell the new asset apart",
  "existing_id": "<asset-id>",
  "existing_mrn": "mrn://asset/table/postgresql/orders"
}
```

`GET /api/v1/mrn/resolve?mrn=<mrn>` returns what any MRN names, including an MRN in the older form and one an asset used to have; see [MRNs in the UI, CLI and API](../MRNs/index.md#mrns-in-the-ui-cli-and-api).
