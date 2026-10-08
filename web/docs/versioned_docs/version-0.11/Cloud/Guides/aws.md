---
sidebar_position: 2
title: AWS
description: Your first pipeline on Marmot Cloud, cataloging the Glue Data Catalog with no access key.
---

import { CalloutCard } from '@site/src/components/DocCard';
import { Steps, Step } from '@site/src/components/Steps';

# AWS

This guide takes you from an empty instance to a pipeline that catalogs your Glue Data Catalog every six hours. The pipeline assumes an IAM role as itself, so there is no access key to create, store or rotate.

You need a Marmot Cloud instance, written as `https://acme.marmotdata.cloud` throughout, an account where you can create IAM roles and identity providers, and Terraform with the `marmot` and `aws` providers. If you have not signed in from the CLI yet, do [getting started](../getting-started.md) first.

## How it works

Your instance is an OIDC issuer. Registered once as an IAM identity provider, it becomes something roles in your account can trust. A pipeline then assumes a role with its own token. Each role's trust policy names one pipeline, so a role is assumable by that pipeline and nothing else.

## Your first pipeline

<Steps>
<Step title="Configure the providers">

```hcl
terraform {
  required_version = ">= 1.11"

  required_providers {
    marmot = { source = "marmotdata/marmot" }
    aws    = { source = "hashicorp/aws" }
  }
}

provider "marmot" {
  host = "https://acme.marmotdata.cloud"
}

provider "aws" {
  region = "eu-west-1"
}

locals {
  marmot_issuer = "https://acme.marmotdata.cloud"
  marmot_host   = "acme.marmotdata.cloud"
}
```

The identity provider takes the full URL. Trust policy conditions take the bare host. A condition written with the scheme never matches and produces no error, which is why both live in locals.

</Step>
<Step title="Trust your instance">

One provider per Marmot instance per account.

```hcl
resource "aws_iam_openid_connect_provider" "marmot" {
  url            = local.marmot_issuer
  client_id_list = ["sts.amazonaws.com"]
}
```

</Step>
<Step title="Create a role for the pipeline">

The trust policy pins the subject to the pipeline's name. The name lives in a local because the policy needs it before the pipeline exists.

```hcl
locals {
  lake_pipeline_name = "lake"
}

data "aws_iam_policy_document" "marmot_lake_trust" {
  statement {
    actions = ["sts:AssumeRoleWithWebIdentity"]

    principals {
      type        = "Federated"
      identifiers = [aws_iam_openid_connect_provider.marmot.arn]
    }

    condition {
      test     = "StringEquals"
      variable = "${local.marmot_host}:sub"
      values   = ["pipeline:${local.lake_pipeline_name}"]
    }

    condition {
      test     = "StringEquals"
      variable = "${local.marmot_host}:aud"
      values   = ["sts.amazonaws.com"]
    }
  }
}

resource "aws_iam_role" "marmot_lake" {
  name               = "marmot-lake"
  assume_role_policy = data.aws_iam_policy_document.marmot_lake_trust.json
}

resource "aws_iam_role_policy" "marmot_lake_read" {
  role = aws_iam_role.marmot_lake.name

  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [{
      Effect   = "Allow"
      Action   = ["glue:GetDatabases", "glue:GetTables", "glue:GetJobs", "glue:GetCrawlers"]
      Resource = "*"
    }]
  })
}
```

Keep the subject condition an exact match. A wildcard would let every future pipeline assume this role.

</Step>
<Step title="Declare the pipeline">

Naming a role in the credentials block is what tells the plugin to assume it as the pipeline rather than look for a key.

```hcl
resource "marmot_pipeline" "lake" {
  name      = local.lake_pipeline_name
  plugin_id = "glue"

  config = jsonencode({
    credentials = {
      region   = "eu-west-1"
      role_arn = aws_iam_role.marmot_lake.arn
    }
  })

  cron_expression = "0 */6 * * *"
}
```

</Step>
<Step title="Apply and run">

```bash
terraform apply
marmot runs list
```

The pipeline runs on its next tick, or now from the Pipelines page in your instance. When the run completes, your Glue databases and tables are in the catalog.

</Step>
</Steps>

Every other AWS plugin works the same way: a role per pipeline with read-only permissions for that service. S3 needs `s3:ListAllMyBuckets` and `s3:GetBucketLocation`. Each plugin's page on the [plugin registry](https://plugins.marmotdata.io) lists its actions.

## When a source needs a password

An RDS or self-managed Postgres cannot federate, so its pipeline needs a real password. Keep it in Secrets Manager and let Marmot read it at run time through a secret store. The store assumes a role of its own, named after the store.

```hcl
locals {
  secret_store_name = "aws-prod"
}

data "aws_iam_policy_document" "marmot_store_trust" {
  statement {
    actions = ["sts:AssumeRoleWithWebIdentity"]

    principals {
      type        = "Federated"
      identifiers = [aws_iam_openid_connect_provider.marmot.arn]
    }

    condition {
      test     = "StringEquals"
      variable = "${local.marmot_host}:sub"
      values   = ["secretStore:${local.secret_store_name}"]
    }

    condition {
      test     = "StringEquals"
      variable = "${local.marmot_host}:aud"
      values   = ["sts.amazonaws.com"]
    }
  }
}

resource "aws_iam_role" "marmot_store" {
  name               = "marmot-secret-store"
  assume_role_policy = data.aws_iam_policy_document.marmot_store_trust.json
}

resource "aws_secretsmanager_secret" "orders_db_password" {
  name = "prod/orders/db-password"
}

data "aws_iam_policy_document" "marmot_reads_orders_password" {
  statement {
    actions   = ["secretsmanager:GetSecretValue"]
    resources = [aws_secretsmanager_secret.orders_db_password.arn]
  }
}

resource "aws_iam_role_policy" "marmot_reads_orders_password" {
  role   = aws_iam_role.marmot_store.name
  policy = data.aws_iam_policy_document.marmot_reads_orders_password.json
}

resource "marmot_secret_store_aws" "prod" {
  name     = local.secret_store_name
  role_arn = aws_iam_role.marmot_store.arn
}

resource "marmot_secret_store_aws_secret" "orders_db_password" {
  store     = marmot_secret_store_aws.prod.id
  secret_id = aws_secretsmanager_secret.orders_db_password.arn
}

resource "marmot_pipeline" "orders" {
  name      = "orders"
  plugin_id = "postgresql"

  config = jsonencode({
    host     = "orders-db.acme.internal"
    database = "orders"
    user     = "marmot"
  })

  secrets = {
    password = marmot_secret_store_aws_secret.orders_db_password.id
  }

  cron_expression = "0 * * * *"
}
```

Name the secret ARN, not a prefix. No version is pinned, so rotating in Secrets Manager is the whole rotation. [Secret stores](../secret-stores.md) covers the model.

## Troubleshooting

| Symptom | Cause |
| --- | --- |
| Access denied on assume role | The trust policy did not match: a renamed pipeline, or a condition written with the scheme. |
| Invalid identity token | The provider URL does not exactly match your instance URL. |
| Role assumed, API call denied | The role's permission policy is missing an action the plugin needs. |

<CalloutCard
  title="Next: decide who sees what"
  description="Grant roles per asset, data product, glossary term and secret store."
  docId="Cloud/access-control"
  buttonText="Access control"
  icon="mdi:shield-key"
/>
