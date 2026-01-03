The Google Cloud Storage plugin discovers buckets from GCP projects. It captures bucket metadata including location, storage class, encryption settings, and lifecycle rules.

## Connection Examples

import { Collapsible } from "@site/src/components/Collapsible";

<Collapsible title="Service Account File" icon="logos:google-cloud">

```yaml
project_id: "my-gcp-project"
credentials_file: "/path/to/service-account.json"
include_metadata: true
tags:
  - "gcs"
  - "storage"
```

</Collapsible>

<Collapsible title="Service Account JSON" icon="mdi:key">

```yaml
project_id: "my-gcp-project"
credentials_json: "${GCS_CREDENTIALS_JSON}"
include_metadata: true
include_object_count: false
filter:
  include:
    - "^data-.*"
  exclude:
    - ".*-temp$"
tags:
  - "gcs"
```

</Collapsible>

## Required Permissions

The service account needs the following IAM roles:

- **Storage Object Viewer** (`roles/storage.objectViewer`) - For listing buckets and objects

Or use a custom role with these permissions:
- `storage.buckets.list`
- `storage.buckets.get`
- `storage.objects.list` (if using object count)
