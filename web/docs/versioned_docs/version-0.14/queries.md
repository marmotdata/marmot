---
sidebar_position: 5
title: Search and the query language
---

import { ThemedImg } from '@site/src/components/ThemedImg';

# Search and the query language

Free text finds most things on **Discover**. The query language narrows a search precisely, such as every Kafka topic a team owns or every table with more than a million rows, and the same queries define [data product rules](data-products.md) and [asset rules](asset-rules.md).

## Build a query

On **Discover**, select **Build query** to pick a field, an operator and values without writing the syntax. **All**, **Any** and **None** set how conditions combine, and the builder counts the matches before you select **Apply**. The query then appears in the search bar as tokens, and **Show as text** shows it as text you can copy or bookmark:

<ThemedImg
  lightSrc="/img/query-builder-light.png"
  darkSrc="/img/query-builder-dark.png"
  alt="Build query on Discover with the condition Provider is Kafka, matching 6 results, and the Apply button"
  size="panel"
/>

## Syntax reference

### Fields

Filter assets using field prefixes:

| Field | Description | Example |
| ----- | ----------- | ------- |
| `@type` | Asset type | `@type: "table"` |
| `@provider` | Provider or platform | `@provider: "kafka"` |
| `@name` | Asset name | `@name: "users"` |
| `@kind` | Resource kind in Marmot | `@kind: "asset"` |
| `@tag` | A tag on the asset | `@tag: "pii"` |
| `@term` | A glossary term linked to the asset | `@term: "GMV"` |
| `@metadata.*` | Custom metadata fields | `@metadata.team: "platform"` |

Metadata supports dot notation for nested fields: `@metadata.config.retention: "7d"`

### Operators

| Operator | Description | Example |
| -------- | ----------- | ------- |
| `:` or `=` | Exact match | `@type: "table"` |
| `!=` | Not equal | `@metadata.environment != "test"` |
| `contains` | Substring match | `@name contains "customer"` |
| `>` `<` `>=` `<=` | Numeric comparison | `@metadata.partitions > 10` |
| `range` | Numeric range | `@metadata.size range [100 TO 500]` |
| `*` | Wildcard | `@name: "customer*"` |

### Boolean logic

Combine filters with `AND`, `OR` and `NOT`. Use parentheses to control precedence:

```marmot
# Multiple conditions
@type: "topic" AND @provider: "kafka"

# Either condition
@metadata.priority: "high" OR @metadata.criticality: "critical"

# Exclusion
@metadata.environment: "production" AND NOT @name contains "test"

# Grouped logic
(@type: "table" OR @type: "view") AND @provider: "postgres"
```

## Examples

import { Collapsible } from '@site/src/components/Collapsible';

<Collapsible title="Free-text search" icon="mdi:text-search" defaultOpen>

Search across asset names, descriptions and metadata without any special syntax.

```marmot
user orders
```

</Collapsible>

<Collapsible title="Filter by type and provider" icon="mdi:filter-variant" defaultOpen>

Find all Kafka topics by combining type and provider filters.

```marmot
@type: "topic" AND @provider: "kafka"
```

</Collapsible>

<Collapsible title="Team ownership" icon="mdi:account-group" defaultOpen>

Find all assets owned by a specific team using custom metadata.

```marmot
@metadata.team: "data-platform"
```

</Collapsible>

<Collapsible title="Numeric comparison" icon="mdi:numeric" defaultOpen>

Filter assets based on numeric metadata values.

```marmot
@type: "topic" AND @metadata.partitions > 10
```

</Collapsible>

<Collapsible title="Wildcard matching" icon="mdi:asterisk" defaultOpen>

Use wildcards when you don't know the exact name.

```marmot
@name: "*customer*" AND @type: "table"
```

</Collapsible>

<Collapsible title="Grouped logic" icon="mdi:code-parentheses" defaultOpen>

Use parentheses to control how conditions are combined.

```marmot
(@type: "table" OR @type: "view") AND @provider: "postgres"
```

</Collapsible>
