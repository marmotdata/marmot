---
sidebar_position: 2
title: Connect AWS
sidebar_label: AWS
description: Your first pipeline on Marmot Cloud, cataloging the Glue Data Catalog with no access key.
---

import { Steps, Step } from '@site/src/components/Steps';
import Tabs from '@theme/Tabs';
import TabItem from '@theme/TabItem';
import { ThemedImg } from '@site/src/components/ThemedImg';

# Connect AWS

Catalog your Glue Data Catalog every 6 hours with a pipeline that assumes an IAM role as itself, so there's no access key to create, store or rotate.

## Before you start

- You hold `ingestion.admin` across the whole catalog in your instance.
- You have an AWS account where you can create IAM roles and identity providers.

## How it works

Before each run, your instance gives the pipeline a 5-minute token with the subject `pipeline:<pipeline-name>`, and the pipeline assumes an IAM role with it. Each role's trust policy names one pipeline, so only that pipeline can assume it. [Keyless pipelines](../pipelines.md#how-keyless-pipelines-work) explains the model.

## Values you'll need

The steps use these values. Replace `<instance>` and `<account-id>` with your own:

- **Provider URL**: `https://<instance>.marmotdata.cloud`. Always the `marmotdata.cloud` address, even with a custom domain
- **Audience**: `sts.amazonaws.com`
- **Condition keys**: `<instance>.marmotdata.cloud:sub` and `:aud`, with the bare host, not the URL
- **Pipeline subject**: `pipeline:lake`, matched exactly. A wildcard would let every future pipeline assume the role
- **Role**: `arn:aws:iam::<account-id>:role/marmot-lake`

## Your first pipeline

<Steps>
<Step title="Trust your instance">

Create one IAM OIDC identity provider for your instance in each account.

<Tabs groupId="cloud-method" queryString="cloud-method">
  <TabItem value="console" label="Console" default>

1. In the IAM console, open **Identity providers** and choose **Add provider**.
2. Choose **OpenID Connect**, enter the provider URL and the audience, then choose **Add provider**.

  </TabItem>
  <TabItem value="cli" label="AWS CLI">

```bash
aws iam create-open-id-connect-provider \
  --url https://<instance>.marmotdata.cloud \
  --client-id-list sts.amazonaws.com
```

  </TabItem>
  <TabItem value="terraform" label="Terraform">

```hcl
provider "marmot" {
  host = "https://<instance>.marmotdata.cloud"
}

provider "aws" {
  region = "eu-west-1"
}

resource "aws_iam_openid_connect_provider" "marmot" {
  url            = "https://<instance>.marmotdata.cloud"
  client_id_list = ["sts.amazonaws.com"]
}
```

  </TabItem>
</Tabs>

</Step>
<Step title="Create a role for the pipeline">

Create a role called `marmot-lake` that only the `lake` pipeline can assume, with read-only Glue access.

<Tabs groupId="cloud-method" queryString="cloud-method">
  <TabItem value="console" label="Console" default>

1. In the IAM console, open **Roles** and choose **Create role**.
2. Choose **Web identity**, then your instance's provider and the audience, and choose **Next**.
3. Choose **Next** again, enter `marmot-lake` as the **Role name** and choose **Create role**.
4. On the role's **Trust relationships** tab, choose **Edit trust policy** and add `"<instance>.marmotdata.cloud:sub": "pipeline:lake"` to the `StringEquals` condition.
5. On the **Permissions** tab, add an inline policy allowing `glue:GetDatabases`, `glue:GetTables`, `glue:GetJobs` and `glue:GetCrawlers` on `*`.

  </TabItem>
  <TabItem value="cli" label="AWS CLI">

Save the trust policy as `trust.json`, with your provider's ARN, `arn:aws:iam::<account-id>:oidc-provider/<instance>.marmotdata.cloud`, in place of `<provider-arn>`:

```json
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Principal": { "Federated": "<provider-arn>" },
      "Action": "sts:AssumeRoleWithWebIdentity",
      "Condition": {
        "StringEquals": {
          "<instance>.marmotdata.cloud:aud": "sts.amazonaws.com",
          "<instance>.marmotdata.cloud:sub": "pipeline:lake"
        }
      }
    }
  ]
}
```

Save the permissions as `glue-read.json`:

```json
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Action": [
        "glue:GetDatabases",
        "glue:GetTables",
        "glue:GetJobs",
        "glue:GetCrawlers"
      ],
      "Resource": "*"
    }
  ]
}
```

Create the role and attach the permissions:

```bash
aws iam create-role \
  --role-name marmot-lake \
  --assume-role-policy-document file://trust.json

aws iam put-role-policy \
  --role-name marmot-lake \
  --policy-name glue-read \
  --policy-document file://glue-read.json
```

  </TabItem>
  <TabItem value="terraform" label="Terraform">

The name lives in a local because the trust policy needs it before the pipeline exists.

```hcl
locals {
  marmot_host        = "<instance>.marmotdata.cloud"
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
  name = "glue-read"
  role = aws_iam_role.marmot_lake.name

  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [{
      Effect = "Allow"
      Action = [
        "glue:GetDatabases",
        "glue:GetTables",
        "glue:GetJobs",
        "glue:GetCrawlers",
      ]
      Resource = "*"
    }]
  })
}
```

  </TabItem>
</Tabs>

</Step>
<Step title="Create the pipeline">

Point the Glue plugin at the role. The role ARN is what tells the plugin to sign in as the pipeline rather than look for a key, and it needs a region.

<Tabs groupId="method" queryString>
  <TabItem value="ui" label="UI" default>

1. Open **Plugins** and select **AWS Glue** under **Add a source**.
2. Enter `lake` as the **Pipeline name** and select **Continue**.
3. Under **Credentials**, turn off **Use Default**, set **Role ARN (web identity)** to the role and **Region** to `eu-west-1`, then select **Check and continue**.
4. Turn on **Run on a schedule**, set it to `0 */6 * * *` and select **Create pipeline**.

<ThemedImg
  lightSrc="/img/cloud-glue-role-light.png"
  darkSrc="/img/cloud-glue-role-dark.png"
  alt="The AWS Glue pipeline's credentials with Use Default off, Role ARN (web identity) set to the marmot-lake role and Region eu-west-1"
/>

  </TabItem>
  <TabItem value="terraform" label="Terraform">

```hcl
resource "marmot_pipeline" "lake" {
  name            = local.lake_pipeline_name
  plugin_id       = "glue"
  cron_expression = "0 */6 * * *"

  config = jsonencode({
    credentials = {
      region   = "eu-west-1"
      role_arn = aws_iam_role.marmot_lake.arn
    }
  })
}
```

  </TabItem>
</Tabs>

</Step>
</Steps>

## Check that it worked

The pipeline runs at its next scheduled time. To run it now, open it under **Plugins** and select **Run now**. When the run succeeds, your Glue databases and tables appear in **Discover**.

From the CLI, `marmot runs list --pipelines lake` lists its runs.

CloudTrail records each `AssumeRoleWithWebIdentity` call with the session name `marmot` and the pipeline's subject.

## Other AWS sources

Every other AWS plugin works the same way: a role per pipeline with read-only permissions for that service. S3 needs at least `s3:ListAllMyBuckets` and `s3:GetBucketLocation`. Each plugin's page in the [plugin directory](https://plugins.marmotdata.io) lists its configuration.

## When a source needs a password

An RDS or self-managed Postgres can't federate, so its pipeline needs a real password. Keep it in Secrets Manager and let Marmot read it at run time through a [secret store](../secret-stores.md). The store assumes a role of its own, as the subject `secretStore:<store-name>`, through the same identity provider.

<Steps>
<Step title="Create a role for the store">

The store will be called `aws-prod`, so its role trusts the subject `secretStore:aws-prod` and allows `secretsmanager:GetSecretValue` on the one secret.

<Tabs groupId="cloud-method" queryString="cloud-method">
  <TabItem value="console" label="Console" default>

1. Create a role called `marmot-secret-store` as in the first pipeline, with **Web identity**, your instance's provider and the audience.
2. On the **Trust relationships** tab, choose **Edit trust policy** and add `"<instance>.marmotdata.cloud:sub": "secretStore:aws-prod"` to the `StringEquals` condition.
3. On the **Permissions** tab, add an inline policy that allows `secretsmanager:GetSecretValue` on the secret's ARN.

  </TabItem>
  <TabItem value="cli" label="AWS CLI">

Save a trust policy as `store-trust.json`, the same as the first pipeline's `trust.json` but with the subject `secretStore:aws-prod`. Save the permissions as `read-orders-password.json`:

```json
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Action": "secretsmanager:GetSecretValue",
      "Resource": "<secret-arn>"
    }
  ]
}
```

```bash
aws iam create-role \
  --role-name marmot-secret-store \
  --assume-role-policy-document file://store-trust.json

aws iam put-role-policy \
  --role-name marmot-secret-store \
  --policy-name read-orders-password \
  --policy-document file://read-orders-password.json
```

  </TabItem>
  <TabItem value="terraform" label="Terraform">

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

resource "aws_iam_role_policy" "marmot_reads_orders_password" {
  name = "read-orders-password"
  role = aws_iam_role.marmot_store.name

  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [{
      Effect   = "Allow"
      Action   = "secretsmanager:GetSecretValue"
      Resource = aws_secretsmanager_secret.orders_db_password.arn
    }]
  })
}
```

  </TabItem>
</Tabs>

</Step>
<Step title="Create the store and register the secret">

Create a store named `aws-prod` that assumes the role, then register the secret in it. A full ARN carries the region; a secret given by name also needs a **Region**.

<Tabs groupId="method" queryString>
  <TabItem value="ui" label="UI" default>

1. Open **Settings → Secret stores** and select **New secret store**.
2. Enter `aws-prod` as the **Name**.
3. Under **Where do the secrets live?**, choose **AWS Secrets Manager**.
4. Set **Role ARN** to `arn:aws:iam::<account-id>:role/marmot-secret-store` and select **Create secret store**.
5. On the store's page, select **Register secret**, enter the secret's ARN as the **Secret ID**, then select **Register secret**.

  </TabItem>
  <TabItem value="terraform" label="Terraform">

```hcl
resource "marmot_secret_store_aws" "prod" {
  name     = local.secret_store_name
  role_arn = aws_iam_role.marmot_store.arn
}

resource "marmot_secret_store_aws_secret" "orders_db_password" {
  store     = marmot_secret_store_aws.prod.id
  secret_id = aws_secretsmanager_secret.orders_db_password.arn
}
```

  </TabItem>
</Tabs>

</Step>
<Step title="Use the secret in a pipeline">

Create the PostgreSQL pipeline with its password read from the store. The store reads the `AWSCURRENT` stage unless you pin another, so rotating in Secrets Manager is the whole rotation.

<Tabs groupId="method" queryString>
  <TabItem value="ui" label="UI" default>

1. Open **Plugins** and select **PostgreSQL** under **Add a source**.
2. Enter `orders` as the **Pipeline name** and select **Continue**.
3. Fill in **Host** and **User**. For **Password**, choose **From secret store** and pick the secret.
4. Select **Check and continue**, set a schedule and select **Create pipeline**.

  </TabItem>
  <TabItem value="terraform" label="Terraform">

```hcl
resource "marmot_pipeline" "orders" {
  name            = "orders"
  plugin_id       = "postgresql"
  cron_expression = "0 * * * *"

  config = jsonencode({
    host = "orders-db.example.com"
    user = "marmot_reader"
  })

  secrets = {
    password = marmot_secret_store_aws_secret.orders_db_password.id
  }
}
```

  </TabItem>
</Tabs>

</Step>
</Steps>

## Troubleshooting

| What you see | What to do |
| --- | --- |
| `region is required with role_arn; ...` | Set **Region** next to the role ARN. |
| `AccessDenied` from `AssumeRoleWithWebIdentity` | The trust policy didn't match. Check the condition keys use the bare host and the subject names the pipeline's current name. |
| `InvalidIdentityToken` | The identity provider's URL doesn't exactly match the provider URL above. |
| The role is assumed but an API call is denied | Add the missing action to the role's permissions policy. |

## Next steps

- [Grant and check access](../grant-access.md): decide who can see the new tables.
- [Secret stores](../secret-stores.md): more about stores, Vault and rotation.
- [Connect Google Cloud](google.md) or [Connect Azure](azure.md) if you catalog more than one cloud.
