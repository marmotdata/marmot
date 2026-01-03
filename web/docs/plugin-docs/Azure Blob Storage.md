The Azure Blob Storage plugin discovers containers from Azure Storage accounts. It captures container metadata including access levels, lease status, and custom metadata.

## Connection Examples

import { Collapsible } from "@site/src/components/Collapsible";

<Collapsible title="Connection String" icon="logos:microsoft-azure">

```yaml
connection_string: "${AZURE_STORAGE_CONNECTION_STRING}"
include_metadata: true
tags:
  - "azure"
  - "storage"
```

</Collapsible>

<Collapsible title="Account Name and Key" icon="mdi:key">

```yaml
account_name: "mystorageaccount"
account_key: "${AZURE_STORAGE_ACCOUNT_KEY}"
include_metadata: true
include_blob_count: false
filter:
  include:
    - "^data-.*"
  exclude:
    - ".*-temp$"
tags:
  - "azure"
```

</Collapsible>

## Required Permissions

The following Azure RBAC role is recommended:

- **Storage Blob Data Reader** - Read access to containers and blobs

Or use a custom role with these permissions:

- `Microsoft.Storage/storageAccounts/blobServices/containers/read`
- `Microsoft.Storage/storageAccounts/blobServices/containers/blobs/read`
