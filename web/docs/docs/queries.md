---
sidebar_position: 5
---

# Query Language

Marmot provides a query language for searching and filtering assets in your catalog. The language supports free-text search, field-specific filters, comparison operators, and boolean logic.

## Free-Text Search

Free-text queries search across asset names, descriptions, and metadata:

```
user orders
```

For more precise results, use field filters.

## Field Filters

Field filters allow you to query specific attributes of assets using prefixes:

### Asset Type (`@type`)

Filter assets by their type:

```
@type: "table"
@type: "topic"
@type: "bucket"
```

### Provider (`@provider`)

Filter assets by their provider or platform:

```
@provider: "kafka"
@provider: "postgres"
@provider: "s3"
```

### Asset Name (`@name`)

Filter by asset name:

```
@name: "users"
@name contains "order"
@name: "customer*"
```

### Kind (`@kind`)

Filter by the type of resource in Marmot:

```
@kind: "asset"
@kind: "glossary"
@kind: "team"
```

### Custom Metadata (`@metadata.*`)

Query any custom metadata field you've added:

```
@metadata.team: "data-platform"
@metadata.environment: "production"
@metadata.owner: "alice@company.com"
@metadata.region: "eu-west-1"
```

You can access nested metadata using dot notation:

```
@metadata.config.retention: "7d"
@metadata.tags.compliance: "pii"
```

## Operators

Marmot supports various operators for precise filtering:

| Operator   | Description           | Example                           |
| ---------- | --------------------- | --------------------------------- |
| `:` or `=` | Exact match           | `@type: "table"`                  |
| `contains` | Substring match       | `@name contains "customer"`       |
| `!=`       | Not equal             | `@metadata.environment != "test"` |
| `>`        | Greater than          | `@metadata.partitions > 10`       |
| `<`        | Less than             | `@metadata.rows < 1000000`        |
| `>=`       | Greater than or equal | `@metadata.replicas >= 3`         |
| `<=`       | Less than or equal    | `@metadata.retention <= 30`       |

### Wildcards

Use `*` for wildcard matching:

```
@name: "customer*"           # Starts with "customer"
@name: "*-prod"              # Ends with "-prod"
@metadata.team: "*platform*" # Contains "platform"
```

### Range Queries

Query numeric ranges efficiently:

```
@metadata.partitions range [5 TO 20]
@metadata.size range [1000 TO 50000]
```

## Boolean Logic

Combine filters with `AND`, `OR`, and `NOT` for complex queries:

```
# All Kafka topics in production
@type: "topic" AND @provider: "kafka" AND @metadata.environment: "production"

# High-priority or critical assets
@metadata.priority: "high" OR @metadata.criticality: "critical"

# Production assets excluding test data
@metadata.environment: "production" AND NOT @name contains "test"
```

### Complex Queries with Parentheses

Use parentheses to control query logic:

```
(@type: "table" OR @type: "view") AND @provider: "postgres"

@metadata.team: "platform" AND (@metadata.status: "active" OR @metadata.priority: "high")

(@provider: "kafka" OR @provider: "sqs") AND NOT @metadata.environment: "*test*"
```

## Examples

Find all Kafka topics:

```
@type: "topic" AND @provider: "kafka"
```

Find assets owned by a specific team:

```
@metadata.team: "data-platform"
```

Find Kafka topics with more than 10 partitions:

```
@type: "topic" AND @provider: "kafka" AND @metadata.partitions > 10
```

Find non-production tables:

```
@type: "table" AND @metadata.environment != "production"
```

Combine free-text with filters:

```
order @type: "table" OR @name contains "order"
```

Complex boolean query:

```
@metadata.criticality: "high" AND (@metadata.status: "degraded" OR @metadata.alerts > 0)
```

## Query Patterns

**Iterative refinement**: Start with broad queries and add filters to narrow results:

```
# Start with
kafka

# Refine to
@provider: "kafka" AND @metadata.environment: "production"

# Further refine
@provider: "kafka" AND @metadata.environment: "production" AND @metadata.partitions > 5
```

**Wildcard exploration**: Use wildcards when exact names are unknown:

```
@name: "*customer*" AND @type: "table"
```

**Hybrid queries**: Combine free-text with field filters:

```
payment @type: "topic" AND @metadata.team: "payments"
```
