---
sidebar_position: 7
---

# Table Preview

Table preview is an **experimental** feature that allows users to see a sample of rows directly in the asset detail page. Because it requires a live connection back to the source database at query time.

When enabled, table preview:

- Registers the `/api/v1/assets/preview/{id}` API endpoint
- Links discovered assets to their ingestion schedules (used to resolve connection details)
- Shows a **Preview** tab on table and view assets in the UI

## How to enable

### Configuration file

```yaml
experimental:
  table_preview: true
```

### Environment variable

```bash
export MARMOT_EXPERIMENTAL_TABLE_PREVIEW=true
```

### Helm chart

```yaml
config:
  experimental:
    table_preview: true
```
