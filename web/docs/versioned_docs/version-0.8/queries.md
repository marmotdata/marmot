---
sidebar_position: 5
---

import { CalloutCard, FeatureCard, FeatureGrid } from '@site/src/components/DocCard';

# Query Language

Marmot provides a query language for searching and filtering assets in your catalog. The language supports free-text search, field-specific filters, comparison operators and boolean logic.

:::tip Optional but Powerful
The query language is entirely optional. Simple free-text searches work well for everyday discovery. When you need precision, such as finding all Kafka topics owned by a specific team or tables with more than a million rows, the query language gives you that control. Queries are also repeatable and shareable, making it easy to bookmark common searches or share them with your team.
:::

## Where It's Used

The query language powers several features across Marmot:

<FeatureGrid>
  <FeatureCard
    title="Search"
    description="Find assets quickly using free-text or precise field filters in the global search bar."
    icon="mdi:magnify"
  />
  <FeatureCard
    title="Data Products"
    description="Define dynamic rules that automatically include matching assets in your Data Products."
    icon="mdi:package-variant-closed"
  />
</FeatureGrid>

<CalloutCard
  title="Build Dynamic Data Products"
  description="Use the query language to create rules that automatically group related assets. As your catalog grows, matching assets are included automatically."
  href="/docs/data-products"
  buttonText="Learn About Data Products"
  icon="mdi:package-variant-closed"
/>

## Query Builder

The search bar includes a visual query builder that helps you construct queries without memorising the syntax. Click the filter icon to open it, select your field and operator, then enter your value. The builder generates the query syntax automatically.

<img src="/img/query-builder-light.png" alt="Query builder in Marmot" />

## Syntax Reference

### Fields

Filter assets using field prefixes:

| Field | Description | Example |
| ----- | ----------- | ------- |
| `@type` | Asset type | `@type: "table"` |
| `@provider` | Provider or platform | `@provider: "kafka"` |
| `@name` | Asset name | `@name: "users"` |
| `@kind` | Resource kind in Marmot | `@kind: "asset"` |
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

### Boolean Logic

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

<CalloutCard
  title="Need Help with Queries?"
  description="Join our Discord community to ask questions and share tips with other Marmot users."
  href="https://discord.gg/TWCk7hVFN4"
  buttonText="Join Discord"
  variant="secondary"
  icon="mdi:account-group"
/>
