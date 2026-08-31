---
sidebar_position: 1
---

# Marmot Cloud

Marmot Cloud is the hosted, commercially supported distribution of Marmot. It
runs the same catalog, the same API and the same plugins as open source Marmot,
with additional capabilities for organisations that need them.

Everything documented elsewhere in this site applies to Marmot Cloud unchanged.
This section covers only what is specific to it.

## What is different

| | Open source | Marmot Cloud |
|---|---|---|
| Catalog, lineage, glossary, data products | Yes | Yes |
| Plugins, MCP server, SDKs, Terraform provider | Yes | Yes |
| Roles and permissions | Yes | Yes |
| **Access control on individual resources** | — | [Access control](access-control.md) |
| **Editor role** | — | Included |

## Access control

Open source Marmot decides access by permission alone: a principal that holds
`assets:view` can see every asset in the catalog. That is the right default for
a catalog, whose value comes from being findable, and it is what Marmot Cloud
does by default too.

Some catalogs contain assets that not everyone should see, and most catalogs
eventually get a service account that should only reach the handful of assets
its job needs. Marmot Cloud adds a way to say that: grants on specific assets,
data products and glossary terms, for specific users, teams and service
accounts.

Start with [Access control](access-control.md) for the model, and
[Access control with Terraform](access-control-terraform.md) to manage it as
code.
