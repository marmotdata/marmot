---
sidebar_position: 3
toc_max_heading_level: 4
---

# Access control with Terraform

The Marmot Terraform provider manages access grants the same way the Google
provider does, with three resources per resource type. Read
[Access control](access-control.md) first for the model.

## The three resource kinds

For each place a grant can attach — `organization`, `asset`, `data_product`,
`glossary_term` — there are three resources:

| Resource | Owns | Use it when |
|---|---|---|
| `marmot_*_iam_member` | One `(role, member)` pair | Several configurations manage access to the same resource |
| `marmot_*_iam_binding` | One role, all its members | One configuration owns one role |
| `marmot_*_iam_policy` | The entire policy | One configuration owns all access to the resource |

:::warning Do not mix them on the same resource

`_iam_policy` removes anything not in its configuration, including bindings
created by an `_iam_binding` or `_iam_member` elsewhere. The two will overwrite
each other on every apply.
:::

## A worked example

An ETL service account that may read one data product, and an analyst team that
may read one asset and one glossary subtree.

```hcl
terraform {
  required_providers {
    marmot = {
      source = "marmotdata/marmot"
    }
  }
}

provider "marmot" {
  host    = var.marmot_host
  api_key = var.marmot_api_key
}

# A role that grants nothing on its own: it is never assigned
# organization-wide, only on specific resources.
#
# Create it once through the API or the UI; roles are not yet a Terraform
# resource.
locals {
  reader_role = "catalog-reader"
}

# The ETL account reads one data product, and therefore every asset that
# product's rules resolve — including assets added to it later.
resource "marmot_data_product_iam_member" "etl_reads_finance" {
  data_product_id = marmot_data_product.finance.id
  role            = local.reader_role
  member          = "serviceAccount:${var.etl_service_account_id}"
}

# The analyst team reads one asset.
resource "marmot_asset_iam_binding" "analysts_read_orders" {
  asset_id = marmot_asset.orders.id
  role     = local.reader_role
  members  = ["group:${marmot_team.analysts.id}"]
}

# …and the finance vocabulary, which covers every term beneath it.
resource "marmot_glossary_term_iam_binding" "analysts_read_finance_terms" {
  glossary_term_id = marmot_glossary_term.finance.id
  role             = local.reader_role
  members          = ["group:${marmot_team.analysts.id}"]
}
```

## The catalog-wide default

Marmot's default is that everyone can read everything. Stated as Terraform, that
is a single grant at the organization level:

```hcl
resource "marmot_organization_iam_member" "everyone_reads" {
  role   = "user"
  member = "allAuthenticated"
}
```

Removing it is what makes the catalog private, after which principals see only
what they are granted. Do this deliberately: it affects everyone at once.

```hcl
# Administrators keep full access.
resource "marmot_organization_iam_binding" "platform_admins" {
  role    = "admin"
  members = ["group:${marmot_team.platform.id}"]
}
```

:::caution Do not lock yourself out

Apply the admin binding before removing the catalog-wide read grant, and keep at
least one administrator that is not managed by the same apply.
:::

## Authoritative policies

`marmot_iam_policy` is a local data source that renders binding blocks into the
document the `_iam_policy` resources take. It contacts no server.

```hcl
data "marmot_iam_policy" "orders" {
  binding {
    role    = "catalog-reader"
    members = [
      "serviceAccount:${var.etl_service_account_id}",
      "group:${marmot_team.analysts.id}",
    ]
  }

  binding {
    role    = "admin"
    members = ["group:${marmot_team.platform.id}"]
  }
}

resource "marmot_asset_iam_policy" "orders" {
  asset_id    = marmot_asset.orders.id
  policy_data = data.marmot_iam_policy.orders.policy_data
}
```

## Concurrent applies

Every write is made against the version of the policy it was built on, so two
applies touching the same resource cannot silently overwrite each other. The
provider retries a lost race automatically, which is the expected path when
`_iam_member` resources in different configurations manage the same resource.

## Importing existing grants

Import ids mirror the resource path:

```bash
# One member's grant
terraform import marmot_asset_iam_member.etl \
  "asset/{asset id}/roles/{role}/serviceAccount:{service account id}"

# One role's binding
terraform import marmot_asset_iam_binding.readers \
  "asset/{asset id}/roles/{role}"

# A whole policy
terraform import marmot_asset_iam_policy.orders "asset/{asset id}"

# The organization
terraform import marmot_organization_iam_member.everyone \
  "root/roles/user/allAuthenticated"
```

## Checking the result

After an apply, confirm what a principal can actually reach:

```bash
curl "$MARMOT_HOST/api/v1/iam/effective-access?member=serviceAccount:$SA_ID" \
  -H "X-API-Key: $MARMOT_API_KEY"
```

Remember that grants are additive: if this returns more than you expect, look
for a role the principal holds organization-wide rather than for a mistake in
the resource-level grants.
