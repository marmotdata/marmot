---
sidebar_position: 2
title: Domains
description: Organize the catalog into domains and subdomains, give stewards and domain admins a subtree, and scope who may edit what once you turn write enforcement on.
---

# Domains

Domains organize the catalog into a tree of business areas, similar to Collibra communities or Purview collections. Every asset, data product, glossary term and ingestion pipeline belongs to exactly one domain. Domain roles then give people a subtree to run, on top of the instance-wide roles described in [Users, roles and access](access-control.md).

Two layers decide what a principal may do:

1. **The native role** (`assets:manage`, `glossary:manage`…) says *what kind* of change it may make, anywhere.
2. **The domain role** (steward or domain admin) says *where*. It only takes effect once [write enforcement](#write-enforcement) is on.

Reading is open: anyone with `assets:view` still reads every domain. Domains whose content only their members can read are not available yet.

## Enabling domains

Domains are off by default.

```yaml
domains:
  enabled: true
```

```
MARMOT_DOMAINS_ENABLED=true
```

- The tables are created on every start, enabled or not, so turning the flag on and off never changes the database schema.
- Domains need the Postgres search backend. With Elasticsearch enabled, the server refuses to start, and the Helm chart refuses to render.
- With the Helm chart, set `config.domains.enabled: true`.

The flag only turns the feature on. It never restricts anyone by itself: see [write enforcement](#write-enforcement).

## The domain tree

Open **Governance → Domains**.

- Domains nest up to 8 levels. Names are unique among siblings, ignoring case.
- **Unassigned** is a fixed bucket, not part of the tree. Anything without an explicit domain is in it, including everything that existed before you enabled domains.
- A domain can only be deleted when it has no subdomains and nothing assigned to it.
- Each asset, product and term page shows its domain as a chip. With the right to edit the entity, the chip lets you move it.

### Pipelines

An ingestion pipeline has a domain too; set it when you create or edit the pipeline. Assets that pipeline **creates** land in its domain. Assets it ingested before are never moved silently: changing a pipeline's domain asks whether to also move the assets that are still in the previous domain.

### Assigning in bulk

`POST /api/v1/domains/import` assigns existing entities from a metadata field that already names their area, as a dry run unless you pass `apply: true`.

## Searching by domain

Discover has a domain filter, and the query language accepts `@domain`, which always includes the subdomains:

```
@domain:Finance
@domain:"Finance/Payments"
@kind:asset AND @domain = "HR"
@domain:Unassigned
@domain:Finance NOT @domain:"Finance/Payments"
```

- A value can be a domain ID, a name at any depth (every domain with that name matches), or a path of names from the root.
- Unassigned also answers to its name in each UI language (`Sin asignar`), unless a real domain has that name.
- Several `@domain` tokens select any of those domains. `NOT @domain:…` excludes a subtree.
- The domain filter applies to the whole query: an `OR` around an `@domain` token does not widen it.
- The query builder offers `@domain` under simple fields, with the domain paths as values.

## Domain roles

Grant roles from a domain's **Roles** section. A role applies to the domain and everything below it, never above or beside it.

| Role | Grants |
| --- | --- |
| Steward | Edit the assets, products and terms in the subtree |
| Domain admin | What a steward can do, plus create, rename, move and delete subdomains and grant roles in the subtree |
| Reader | Nothing yet; reserved for domains with restricted reading |

- A role can go to a user, a team (every member gets it) or a service account.
- A domain admin can only grant roles inside its own subtree. Top-level domains are created by global administrators only.
- The native `admin` role and the operator token act on every domain.
- A revocation applies from the next request.

## Write enforcement

With enforcement off, which is the default, domain roles are informative and the native roles alone decide who edits. Turn it on once roles are in place.

With enforcement on:

- Editing an asset, product or term needs a steward or domain admin role on its domain, on top of the native permission. This covers REST, MCP, OpenLineage, ingestion runs and lineage.
- Creating needs the role on the destination. The create endpoints (`POST /api/v1/assets/`, `/products/`, `/glossary/`) take an optional `?domain_id=`: the entity is created straight in that domain. Without it, the destination is Unassigned, which is usually reserved for onboarding accounts.
- Moving an entity to another domain needs the role on both its current domain and the target.
- A scheduled ingestion run writes with the scope of its pipeline's domain. A run started over the API also needs the caller to be allowed there, so naming a pipeline grants nothing.
- Lineage edges follow their **target**: the downstream asset declares what it reads, wherever the source lives. This holds for the edges an OpenLineage event creates too: an edge the emitter may not write is skipped with a warning, and the rest of the event is processed.
- Documentation pages follow the asset or product they belong to.
- Asset rules and data product rules match assets anywhere, so only global administrators can change them.

A refused write returns `403 Not allowed in this domain`. The UI hides edit controls the entity's domain does not allow, and domain pickers only offer the domains you can write in.

### Turning it on

Open **Governance → Domains → Write enforcement** (global administrators only). The page lists:

- every user and service account that edits through a native role, outside the `admin` role, with the topmost domains it keeps and the ones it would lose;
- the pipelines whose ingested assets sit outside their domain: their runs could no longer update or remove those assets.

Turning enforcement on confirms the plan you reviewed by its hash. If anything in it changed in the meantime, the request is refused with `plan_changed`: review it again.

The same from the command line:

```
marmot domains enforcement status
marmot domains enforcement plan
marmot domains enforcement enable --confirm <plan hash>
marmot domains enforcement disable
```

## Audit

Every change of domain is recorded, whether enforcement is on or not: creates into a domain, moves of entities and pipelines, domain moves, and turning enforcement on or off.

```
GET /api/v1/domains/audit/{kind}/{id}
```

`kind` is `asset`, `data_product`, `glossary_term`, `ingestion_schedule`, `domain`, or `setting` (with ID `write_enforcement`). Global administrators only.

## API

| Endpoint | Purpose |
| --- | --- |
| `GET /api/v1/domains`, `GET …/{id}`, `GET …/{id}/tree` | Read the tree |
| `POST /api/v1/domains`, `PUT …/{id}`, `DELETE …/{id}`, `POST …/{id}/move` | Manage domains |
| `PUT /api/v1/domains/{id}/members` | Move entities into a domain |
| `GET /api/v1/domains/of/{kind}/{id}` | An entity's domain |
| `GET`, `PUT /api/v1/domains/pipelines/{scheduleId}/assignment` | A pipeline's domain, with `move_assets` |
| `GET`, `POST`, `DELETE /api/v1/domains/{id}/roles` | Domain roles |
| `GET /api/v1/domains/capabilities` | What the caller may do in a domain |
| `GET /api/v1/domains/writable` | The domains the caller may write in |
| `GET /api/v1/domains/enforcement`, `GET …/enforcement/plan`, `POST …/enforcement` | Write enforcement |
| `POST /api/v1/domains/import` | Assign from a metadata field |

Errors carry a stable `code` next to the message, such as `forbidden`, `name_conflict`, `not_empty` or `plan_changed`.

## Known limits

- Elasticsearch search is not supported with domains.
- Reading cannot be restricted per domain yet; `restricted: true` is rejected.
- Asset rules and data product rules are global, and so are the links they derive.
- With `openlineage.auth.enabled: false`, OpenLineage events carry no identity, so under write enforcement they cannot write anything. Keep OpenLineage authentication on and give the emitter's service account a role on the domains it writes.
