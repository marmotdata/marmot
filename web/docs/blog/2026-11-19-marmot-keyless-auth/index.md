---
slug: marmot-keyless-auth
title: "Marmot now supports keyless auth"
authors:
  - name: Bruno Schaatsbergen
    url: https://github.com/bschaatsbergen
image: /img/marmot-oidc-banner.png
description: "Marmot Cloud and Marmot Enterprise support keyless auth. Your instance is an OIDC issuer that Google Cloud, AWS and Azure trust directly, so every pipeline, secret store and service account is a principal you grant to by name, with no key to create, store or rotate."
tags: [marmot-cloud, security, oidc, google-cloud, aws, azure]
keywords:
  - keyless auth
  - keyless data catalog
  - data catalog oidc
  - workload identity federation data catalog
  - data catalog without service account keys
  - marmot cloud security
---

import { CalloutCard } from '@site/src/components/DocCard';
import { ThemedImg } from '@site/src/components/ThemedImg';
import Tabs from '@theme/Tabs';
import TabItem from '@theme/TabItem';

<div style={{textAlign: 'center', marginBottom: '2rem'}}>
  <img src="/img/marmot-oidc-banner.png" alt="The Marmot mascot next to a crossed-out key, over the Plugins page" style={{maxWidth: '100%', borderRadius: '8px'}} />
</div>

Connecting something to BigQuery, Glue and Blob Storage unfortunately often happens through long-lived credentials. A service account key, an access key, a client secret. Each one is a secret that has to be created, stored and rotated, and that lives long enough to leak. Nobody wants to hold them.

Marmot Cloud and Marmot Enterprise implement OIDC to support keyless auth. When a pipeline launches, your instance signs a short-lived token for it, and your cloud trusts that token the way it trusts one from GitHub Actions. The pipeline becomes a principal on Google Cloud, assumes an IAM role on AWS, or signs in as an app registration on Azure. You assign the access by name, and no key is ever created.

<!-- truncate -->

## How keyless auth works

Your instance is an OpenID Connect issuer, like GitHub Actions or a Kubernetes cluster. It publishes its signing key at a well-known address, as [keyless pipelines](/docs/Cloud/pipelines#how-keyless-pipelines-work) explains in full, and each time a pipeline launches it signs a token that names the pipeline and expires in five minutes. Your cloud checks the signature and gives the pipeline the access you assigned to it. Marmot stores no credential.

| Platform | A pipeline becomes |
| --- | --- |
| Google Cloud | A principal: `principal://iam.googleapis.com/<pool>/subject/pipeline:analytics` |
| AWS | A role whose trust policy matches `<instance>.marmotdata.cloud:sub` to `pipeline:lake` |
| Azure | A federated credential with subject `pipeline:blobs` on an app registration or managed identity |
| HashiCorp Vault | A role with `bound_subject` set to `secretStore:vault-prod`, for secret stores |

Setting it up is two steps on your cloud and one in Marmot: trust your instance, give the pipeline something to become, create the pipeline. Pick your cloud in each step. The examples catalog BigQuery, the Glue Data Catalog and a Blob Storage account, with pipelines named `analytics`, `lake` and `blobs`. The guides for [Google Cloud](/docs/Cloud/Guides/google), [AWS](/docs/Cloud/Guides/aws) and [Azure](/docs/Cloud/Guides/azure) have the same steps in the console and in Terraform too.

Everywhere a step asks for the issuer, it is `https://<instance>.marmotdata.cloud` with no trailing slash, even if your instance has a [custom domain](/docs/Cloud/instances#hostnames).

---

## Step 1: Trust your instance

<Tabs groupId="cloud" queryString="cloud">
  <TabItem value="google" label="Google Cloud" default>

Create a Workload Identity pool with one OIDC provider that names your instance as the issuer, and map `google.subject` to the token's subject. Once per project:

```bash
gcloud iam workload-identity-pools create marmot \
  --project=analytics-prod --location=global

gcloud iam workload-identity-pools providers create-oidc marmot \
  --project=analytics-prod --location=global \
  --workload-identity-pool=marmot \
  --issuer-uri="https://<instance>.marmotdata.cloud" \
  --attribute-mapping="google.subject=assertion.sub"
```

The later steps refer to the pool as `$POOL`. It uses the project number, not the project ID:

```bash
POOL=projects/<project-number>/locations/global/workloadIdentityPools/marmot
```

  </TabItem>
  <TabItem value="aws" label="AWS">

Create one IAM OIDC identity provider for your instance, with `sts.amazonaws.com` as the audience. Once per account:

```bash
aws iam create-open-id-connect-provider \
  --url https://<instance>.marmotdata.cloud \
  --client-id-list sts.amazonaws.com
```

Its ARN is `arn:aws:iam::<account-id>:oidc-provider/<instance>.marmotdata.cloud`. The next step needs it.

  </TabItem>
  <TabItem value="azure" label="Azure">

Azure has no shared trust. Each pipeline gets its own app registration, with no client secret and no certificate, and the trust is a federated credential on that registration. Create the registration and its service principal:

```bash
az ad app create --display-name marmot-blob-pipeline \
  --query appId --output tsv

az ad sp create --id <client-id>
```

The first command prints the client ID. A [user-assigned managed identity](/docs/Cloud/Guides/azure#use-a-managed-identity-instead) works in place of the app registration, with `az identity federated-credential create` in the next step.

  </TabItem>
</Tabs>

---

## Step 2: Give the pipeline access

<Tabs groupId="cloud" queryString="cloud">
  <TabItem value="google" label="Google Cloud" default>

The pipeline will be called `analytics`, so in the pool it is the principal `pipeline:analytics`. Grant it BigQuery Metadata Viewer on the project. It reads dataset and table structure, no rows:

```bash
gcloud projects add-iam-policy-binding analytics-prod \
  --role=roles/bigquery.metadataViewer \
  --member="principal://iam.googleapis.com/$POOL/subject/pipeline:analytics"
```

There is no service account in this step and no key.

  </TabItem>
  <TabItem value="aws" label="AWS">

The pipeline will be called `lake`, so it gets a role that only `pipeline:lake` can assume. Save the trust policy as `trust.json`, with the provider ARN from step 1:

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

The condition keys use the bare hostname, and the subject is matched exactly. A wildcard would let every future pipeline assume the role. Save the read-only Glue permissions as `glue-read.json`:

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
  <TabItem value="azure" label="Azure">

The pipeline will be called `blobs`. Add a federated credential to the registration that names your instance as the issuer and `pipeline:blobs` as the subject:

```bash
az ad app federated-credential create --id <client-id> --parameters '{
  "name": "marmot-blobs",
  "issuer": "https://<instance>.marmotdata.cloud",
  "subject": "pipeline:blobs",
  "audiences": ["api://AzureADTokenExchange"]
}'
```

Then assign the registration Storage Blob Data Reader on the storage account:

```bash
SCOPE=$(az storage account show --name ordersarchive \
  --resource-group data-prod --query id --output tsv)

az role assignment create --assignee <client-id> \
  --role "Storage Blob Data Reader" --scope "$SCOPE"
```

Entra ID matches issuer, subject and audience exactly. It accepts a wrong subject when you save it and only fails at the first run.

  </TabItem>
</Tabs>

---

## Step 3: Create the pipeline

In your instance, open **Plugins** and pick the source under **Add a source**. The thing that makes the pipeline keyless is the same on every cloud: you name the identity it should become instead of giving it a key. Every other Google Cloud and AWS plugin works the same way, and the [plugin directory](https://plugins.marmotdata.io) lists each one's settings. On Azure, Blob Storage is the plugin that federates today.

<Tabs groupId="cloud" queryString="cloud">
  <TabItem value="google" label="Google Cloud" default>

1. Select **Google BigQuery**, enter `analytics` as the **Pipeline name** and select **Continue**.
2. Set **Project ID** to `analytics-prod` and **Workload Identity Provider** to `$POOL/providers/marmot`, leave the key fields empty and select **Check and continue**.
3. Turn on **Run on a schedule**, set it to `0 */6 * * *` and select **Create pipeline**.

<ThemedImg
  lightSrc="/img/cloud-bigquery-federated-light.png"
  darkSrc="/img/cloud-bigquery-federated-dark.png"
  alt="The BigQuery pipeline's connection settings with Project ID analytics-prod and the Workload Identity Provider filled in and no key"
/>

  </TabItem>
  <TabItem value="aws" label="AWS">

1. Select **AWS Glue**, enter `lake` as the **Pipeline name** and select **Continue**.
2. Under **Credentials**, turn off **Use Default**, set **Role ARN (web identity)** to `arn:aws:iam::<account-id>:role/marmot-lake` and **Region** to `eu-west-1`, then select **Check and continue**.
3. Turn on **Run on a schedule**, set it to `0 */6 * * *` and select **Create pipeline**.

<ThemedImg
  lightSrc="/img/cloud-glue-role-light.png"
  darkSrc="/img/cloud-glue-role-dark.png"
  alt="The AWS Glue pipeline's credentials with Use Default off, Role ARN (web identity) set to the marmot-lake role and Region eu-west-1"
/>

  </TabItem>
  <TabItem value="azure" label="Azure">

1. Select **Azure Blob Storage**, enter `blobs` as the **Pipeline name** and select **Continue**.
2. Set **Account Name** to `ordersarchive`, **Tenant ID** to your tenant and **Client ID** to the registration's client ID, leave the keys empty and select **Check and continue**.
3. Turn on **Run on a schedule**, set it to `0 */6 * * *` and select **Create pipeline**.

<ThemedImg
  lightSrc="/img/cloud-azureblob-federated-light.png"
  darkSrc="/img/cloud-azureblob-federated-dark.png"
  alt="The Azure Blob Storage pipeline's connection settings with Account Name ordersarchive, a Tenant ID and a Client ID filled in and no key"
/>

  </TabItem>
</Tabs>

---

## Check that it worked

Open the pipeline under **Plugins** and select **Run now**. When the run succeeds, the datasets, tables or containers appear under **Discover**. From the CLI, `marmot runs list --pipelines <name>` lists its runs. Each guide has a [troubleshooting table](/docs/Cloud/Guides/google#troubleshooting) for the errors you are most likely to see.

If the first run is denied right after you granted access, wait a few minutes and run it again. IAM bindings on Google Cloud and role assignments on Azure take a moment to propagate. On AWS, every launch is in CloudTrail as an `AssumeRoleWithWebIdentity` call with the session name `marmot` and the pipeline's subject, so a denied run is in there too, with the subject the pipeline presented.

---

## Sources that need a password

A Postgres on RDS or a Kafka cluster with SASL still wants a password. Keep it in Google Secret Manager, AWS Secrets Manager, Azure Key Vault or HashiCorp Vault, and let a [secret store](/docs/Cloud/secret-stores) read it at launch. The store uses keyless auth too, as `secretStore:<name>`, [trusted the same way](/docs/Cloud/secret-stores#trust-the-store) as a pipeline, and Marmot keeps the address of the secret, never the value. In the pipeline, choose **From secret store** for the password field:

<ThemedImg
  lightSrc="/img/cloud-postgres-secret-light.png"
  darkSrc="/img/cloud-postgres-secret-dark.png"
  alt="A PostgreSQL pipeline's connection settings with the password read from a secret in the gcp-prod store"
/>

Rotating the password is adding a version in your secret manager. The next run picks it up. Each cloud guide walks through the store for that cloud, and the [secret stores](/docs/Cloud/secret-stores#register-a-store) page covers Vault, roles and rotation.

---

## Nothing to rotate

Keyless auth is available on Marmot Cloud and Marmot Enterprise today, for the Google Cloud, AWS and Azure plugins that can federate and for secret stores on Google Secret Manager, AWS Secrets Manager, Azure Key Vault and HashiCorp Vault. Once your cloud trusts your instance, adding a source is a grant on your side and a pipeline on ours. No key is created, so there is nothing to rotate, nothing to leak and nothing to find in a config file a year from now.

<CalloutCard
  title="Connect your cloud"
  description="The full guides for Google Cloud, AWS and Azure, with every step in the console, the CLI and Terraform."
  href="/docs/Cloud/Guides/google"
  buttonText="Open the guide"
  icon="mdi:cloud-sync"
/>

<CalloutCard
  title="Join the Community"
  description="Get help, share feedback and connect with other Marmot users on Discord."
  href="https://discord.gg/TWCk7hVFN4"
  buttonText="Join Discord"
  variant="secondary"
  icon="mdi:account-group"
/>
