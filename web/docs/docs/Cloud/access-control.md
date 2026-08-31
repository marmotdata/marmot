---
sidebar_position: 2
toc_max_heading_level: 4
---

# Access control

By default every authenticated principal in Marmot can see every asset, every
lineage edge and every glossary term. Access control lets you narrow that for
particular principals: a service account that may read three assets, a team that
may read one data product, an analyst who may read one glossary subtree.

The model follows Google Cloud IAM. If you have written `google_project_iam_*`
resources before, this will be familiar.

## The model

A **grant** binds a **role** to a **member** on a **resource**.

```
  role          a bundle of permissions — the existing Marmot roles
  member        who holds it — a user, team, service account, or everyone
  resource      where it applies — the organization, an asset, a data product,
                or a glossary term
```

Grants are **additive**, and there are **no denies**. A principal's effective
access is the union of every grant that reaches it. This has one consequence
worth internalising before you start:

:::warning Adding a narrow grant does not narrow anything

If a service account already holds `assets:view` over the whole catalog — which
the default `user` role gives it — then granting it access to one asset changes
nothing. It could already see that asset, and everything else.

To restrict a principal you give it a role that does **not** carry the
permission organization-wide, and then grant that role on specific resources.
:::

### The hierarchy

```
organization                    the whole catalog
├── data product                and every asset its rules resolve
├── asset
└── glossary term               and its descendant terms
```

Grants are inherited downward. A grant on the organization applies everywhere. A
grant on a data product reaches every asset that product resolves. A grant on a
glossary term reaches the terms beneath it.

An asset can belong to several data products, or none. Access to an asset is the
union of the grants on the organization, on every data product containing it,
and on the asset itself.

### Members

Members are written the way Google writes them:

| Member | Meaning |
|---|---|
| `user:{user id}` | One user |
| `group:{team id}` | Everyone in a Marmot team, including members synced from your identity provider |
| `serviceAccount:{service account id}` | One service account |
| `allAuthenticated` | Every principal that got past authentication |

`group:` is how SSO-driven access works: sync the identity provider group to a
Marmot team, then grant the team.

`allAuthenticated` is how the default — everyone sees everything — is stated
explicitly, and removing that grant is what locks an instance down.

### Roles

Grants use the roles you already have. Create a role holding exactly the
permissions a scoped principal should have, assign it to nobody
organization-wide, and grant it where it is needed.

Marmot ships no role for this — `admin`, `editor` and `user` are the only
built-in roles, and none of them is a scoped reader. That is deliberate: built-in
roles are system roles, and system roles cannot be edited, so a shipped
"catalog reader" would be a permission set you could never tune. Whether a
scoped reader should also see lineage neighbours, or glossary terms, is a
decision only you can make. `catalog-reader` below is a name used throughout
these docs for a role you create yourself.

```bash
# A role for principals whose access is granted per resource.
curl -X POST "$MARMOT_HOST/api/v1/roles" \
  -H "X-API-Key: $MARMOT_API_KEY" \
  -H 'Content-Type: application/json' \
  -d '{
        "name": "catalog-reader",
        "description": "Read access, granted on specific resources",
        "permission_ids": ["<view_assets id>", "<view_lineage_neighbors id>"]
      }'
```

`GET /api/v1/permissions` lists the available permissions and their ids.

## Granting access

Policies are read and written whole, per resource.

```bash
# Read the current policy on an asset.
curl "$MARMOT_HOST/api/v1/iam/asset/$ASSET_ID/policy" \
  -H "X-API-Key: $MARMOT_API_KEY"
```

```json
{
  "etag": "e3b0c44298fc1c149afbf4c8996fb924",
  "bindings": [
    {
      "role": "catalog-reader",
      "members": ["serviceAccount:6f0c…"]
    }
  ]
}
```

```bash
# Replace it. The etag must be the one you just read.
curl -X PUT "$MARMOT_HOST/api/v1/iam/asset/$ASSET_ID/policy" \
  -H "X-API-Key: $MARMOT_API_KEY" \
  -H 'Content-Type: application/json' \
  -d '{
        "policy": {
          "etag": "e3b0c44298fc1c149afbf4c8996fb924",
          "bindings": [
            {"role": "catalog-reader", "members": ["serviceAccount:6f0c…"]}
          ]
        }
      }'
```

Writing with a stale etag returns `409`. That is deliberate: without it, two
people editing the same policy at the same time would silently discard each
other's changes. Re-read the policy and apply your change to the result.

The resource in the path is one of `root`, `asset`, `data_product` or
`glossary_term`. For `root`, the id is `-`:

```
GET  /api/v1/iam/root/-/policy
GET  /api/v1/iam/asset/{id}/policy
GET  /api/v1/iam/data_product/{id}/policy
GET  /api/v1/iam/glossary_term/{id}/policy
```

Reading a policy requires `iam:view`; writing one requires `iam:manage`. Both
belong to `admin` by default.

## Working out why access is what it is

Because grants are additive, a surprising answer is almost always a broader
grant somewhere above the one you are looking at. Two endpoints answer this
directly.

**What does this member actually hold on this resource?**

```bash
curl -X POST "$MARMOT_HOST/api/v1/iam/asset/$ASSET_ID/test-permissions" \
  -H "X-API-Key: $MARMOT_API_KEY" \
  -H 'Content-Type: application/json' \
  -d '{"member": "serviceAccount:6f0c…", "permissions": ["assets:view", "assets:manage"]}'
```

```json
{"permissions": ["assets:view"]}
```

Omit `member` to test your own access, which needs no special permission.

**Which grants does this member have, and where?**

```bash
curl "$MARMOT_HOST/api/v1/iam/effective-access?member=serviceAccount:6f0c…" \
  -H "X-API-Key: $MARMOT_API_KEY"
```

```json
{
  "member": "serviceAccount:6f0c…",
  "admin": false,
  "roles": ["catalog-reader"],
  "grants": [
    {
      "resource_type": "asset",
      "resource_id": "1f0c6e9a…",
      "role": "catalog-reader",
      "permissions": ["assets:view", "lineage:view_neighbors"],
      "inherited": false
    }
  ]
}
```

## Lineage

Lineage is a graph, so restricting it raises a question the rest of the catalog
does not: what happens at the edge of what you can see?

Marmot's answer is a **one-hop frontier**. A principal sees its own assets in
full, and their immediate neighbours as redacted placeholders — no name, no MRN,
no type, no tags, no metadata, just an opaque identifier that edges can attach
to. Redacted nodes are never expanded, so the graph stops exactly one hop past
what you can see, no matter what depth you ask for.

```
   [redacted] ──▶ orders_public ──▶ [redacted]        you can see orders_public
                                          │
                                          ✗           two hops away is not returned
```

This keeps impact analysis working — *something upstream feeds this, something
downstream depends on it* — without disclosing what those things are.

This behaviour comes from the `lineage:view_neighbors` permission. A role
without it gets strict mode instead: only edges whose endpoints are both
visible, and no placeholders at all. Strict mode leaks nothing, at the cost of
showing a scoped principal a disconnected graph.

The default `user` and `editor` roles hold `lineage:view_neighbors`, so nothing
changes for ordinary users.

## Glossary

A grant on a glossary term covers its descendants, so binding the top of a
subtree grants the vocabulary beneath it in one go.

Inheritance runs downward only. A grant on a leaf term does not disclose the
terms above it — the breadcrumb is filtered to what you can see.

Terms remain readable by everyone by default, because the default `user` role
holds `glossary:view` organization-wide.

## Things worth knowing

### A resource you cannot see returns 404

Not `403`. In a catalog the names are frequently the sensitive part, and a `403`
would confirm that an asset exists. Endpoints that cannot filter what they
return refuse a scoped principal outright rather than over-serving it.

### Data product membership is defined by a query

A data product's rules decide which assets inherit its grants, so whoever can
rewrite those rules can widen access. Once a data product carries any grant,
changing its rules or its manual membership requires `iam:manage`, not just
`assets:manage`. Data products with no grants are unaffected.

### Access is scoped for reading, not writing

Grants scope what a principal can **see**. Write permissions — creating and
editing assets, lineage and glossary terms — are still organization-wide: a
grant on one asset never opens a write route over the whole catalog.

### Revocation is not instantaneous

Effective access is cached briefly. A change made through the API takes effect
within a few seconds across every replica. A change made directly in the
database takes the same few seconds.

### The operator is not scoped

The Kubernetes operator authenticates with a cluster service-account token and
acts as an administrator. It is the one principal grants do not apply to.
