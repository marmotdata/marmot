The Google Cloud Storage plugin discovers buckets from GCP projects. It captures bucket metadata including location, storage class, encryption settings, and lifecycle rules.

## Authentication

import { Steps, Step, Tabs, TabPanel, TipBox } from '@site/src/components/Steps';

The recommended approach is keyless: **[Application Default Credentials](https://cloud.google.com/docs/authentication/application-default-credentials)**. The model is the same everywhere: you grant a single service account (e.g., `marmot@my-gcp-project.iam.gserviceaccount.com`) permission to list buckets, then run Marmot _as_ that account.

<Steps>
<Step title="Create the service account and grant it access">

This is shared across every environment.

```bash
# Create the service account
gcloud iam service-accounts create marmot \
  --project my-gcp-project \
  --display-name "Marmot GCS discovery"

# Grant it permission to discover buckets
gcloud projects add-iam-policy-binding my-gcp-project \
  --member "serviceAccount:marmot@my-gcp-project.iam.gserviceaccount.com" \
  --role "roles/storage.bucketViewer"
```

See [Required Permissions](#required-permissions) for the exact role and its alternatives.

</Step>
<Step title="Authenticate as the service account">

Pick the tab for where Marmot runs.

<Tabs items={[
{ label: "GKE", value: "gke", icon: "mdi:kubernetes" },
{ label: "Cloud Run", value: "run", icon: "mdi:google-cloud" },
{ label: "Other platform", value: "external", icon: "mdi:server" }
]}>
<TabPanel>

Bind Marmot's Kubernetes service account to the Google service account with [Workload Identity](https://cloud.google.com/kubernetes-engine/docs/how-to/workload-identity). Credentials resolve automatically, so the plugin config is just:

```yaml
project_id: "my-gcp-project"
include_metadata: true
```

</TabPanel>
<TabPanel>

Attach the service account to the Cloud Run service. The metadata server provides credentials, so they resolve automatically and the plugin config is just:

```yaml
project_id: "my-gcp-project"
include_metadata: true
```

</TabPanel>
<TabPanel>

Running outside Google Cloud, on another cloud, on premises, or in CI? Use [Workload Identity Federation](https://cloud.google.com/iam/docs/workload-identity-federation) so the platform's own identity (an AWS role, an Azure managed identity, or any OIDC or SAML token) can impersonate the service account without an exported key. Create a Google identity pool/provider that trusts your identity provider, grant that external identity the `roles/iam.workloadIdentityUser` role on the service account, then generate a credential configuration file and point `GOOGLE_APPLICATION_CREDENTIALS` at it. If you're running on AWS or Azure, see Google's guides for [VMs](https://docs.cloud.google.com/iam/docs/workload-identity-federation-with-other-clouds#gcloud_1) or [EKS and AKS](https://docs.cloud.google.com/iam/docs/workload-identity-federation-with-kubernetes). The plugin config is then just:

```yaml
project_id: "my-gcp-project"
include_metadata: true
```

</TabPanel>
</Tabs>

</Step>
</Steps>

### Precedence

When multiple credential options are set, the plugin resolves them in this order:

1. `disable_auth`: skips authentication entirely; for local emulators only.
2. `credentials_json`: inline service account key.
3. `credentials_file`: service account key file.
4. **Application Default Credentials**: used when none of the above are set (recommended).

`impersonate_service_account` is independent: whichever identity is resolved above is used only to issue temporary tokens for the target account (the base identity must have `roles/iam.serviceAccountTokenCreator` on the target service account); no key is ever exported.

<TipBox variant="warning" title="Avoid service account keys">
Exported keys are persistent secrets that Google <a href="https://cloud.google.com/iam/docs/best-practices-for-managing-service-account-keys">recommends against</a>.
</TipBox>

## Required Permissions

The service account needs the following IAM roles:

- **Storage Bucket Viewer** (`roles/storage.bucketViewer`) - For discovering and listing buckets

Or use a custom role with these permissions:
- `storage.buckets.list`
- `storage.buckets.get`
- `storage.objects.list` (if using object count)
