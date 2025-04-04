---
sidebar_position: 5
---

# Queries

The query language allows you to search and filter assets using both free text and structured metadata queries.

## Basic Queries

The simplest way to search is using free text:

```
order service
```

## Metadata Filters

To search specific metadata fields, use the `@metadata` prefix:

```
@metadata.team: "logistics"
```

You can combine metadata filters with free text:

```
order @metadata.team: "logistics" @metadata.status: "active"
```

### Operators

The following operators are supported for metadata filters:

| Operator | Description           | Example                                    |
| -------- | --------------------- | ------------------------------------------ |
| : or =   | Exact match           | `@metadata.name: "OrderService"`           |
| contains | Substring match       | `@metadata.description contains "payment"` |
| !=       | Not equal             | `@metadata.environment != "prod"`          |
| >        | Greater than          | `@metadata.partitions > 5`                 |
| &lt;     | Less than             | `@metadata.rows < 1024`                    |
| >=       | Greater than or equal | `@metadata.replicas >= 3`                  |
| &lt;=    | Less than or equal    | `@metadata.partitions <= 2`                |

### Wildcards

Use `*` for wildcard matching:

```
@metadata.name: "order*"
@metadata.environment: "*prod*"
```

### Range Queries

For numeric ranges, use the `range` operator:

```
@metadata.partitions range [1 TO 10]
```

## Boolean Logic

Combine queries using `AND`, `OR`, and `NOT`:

```
@metadata.team: "orders" AND @metadata.environment: "prod"
@metadata.status: "active" OR @metadata.priority: "high"
@metadata.team: "orders" AND NOT @metadata.environment: "test"
```

Use parentheses for complex combinations:

```
(@metadata.team: "orders" AND @metadata.status: "active") OR @metadata.priority: "high"
```

## Examples

Search for production order services:

```
@metadata.type: "service" AND @metadata.name contains "order" AND @metadata.environment: "prod"
```

Find high-partition Kafka topics:

```
@metadata.type: "topic" AND @metadata.partitions > 10
```

Search for non-test services in specific teams:

```
@metadata.type: "service" AND (@metadata.team: "orders" OR @metadata.team: "payments") AND NOT @metadata.environment: "*test*"
```
