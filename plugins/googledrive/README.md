---
title: Google Drive
description: This plugin discovers folders, files and spreadsheets from Google Drive.
status: experimental
---

# Google Drive

<div class="flex flex-col gap-3 mb-6 pb-6 border-b border-gray-200">
<div class="flex items-center gap-3">
<span class="inline-flex items-center rounded-full px-4 py-2 text-sm font-medium bg-earthy-yellow-300 text-earthy-yellow-900">Experimental</span>
</div>
<div class="flex items-center gap-2">
<span class="text-sm text-gray-500">Creates:</span>
<div class="flex flex-wrap gap-2"><span class="inline-flex items-center rounded-lg px-4 py-2 text-sm font-medium bg-earthy-green-100 text-earthy-green-800 border border-earthy-green-300">Assets</span><span class="inline-flex items-center rounded-lg px-4 py-2 text-sm font-medium bg-earthy-green-100 text-earthy-green-800 border border-earthy-green-300">Lineage</span></div>
</div>
</div>

import { CalloutCard } from '@site/src/components/DocCard';

<CalloutCard
  title="Configure in the UI"
  description="This plugin can be configured directly in the Marmot UI with a step-by-step wizard."
  href="/docs/Populating/UI"
  buttonText="View Guide"
  variant="secondary"
  icon="mdi:cursor-default-click"
/>

The Google Drive plugin catalogues the documents and sheets a team keeps in Drive, so the spreadsheet a report is actually built from is findable next to the warehouse tables feeding it.

## What it discovers

| Google Drive | Marmot asset type |
|---|---|
| the drive itself | Drive |
| folder | Folder |
| file | File |
| Google Sheet | Spreadsheet |
| a sheet within a spreadsheet | Table, with its header row as columns |

The drive is the root of the tree: folders nest under it, and each file is linked to the folder holding it, so a drive is navigable from either end: the **Contents** tab on a folder lists what is inside it, and on a file it shows the folder it lives in.

These are the same asset types and names the [OpenMetadata](./OpenMetadata) plugin produces for a Google Drive service. An organisation moving off OpenMetadata can import their drive from there and later point this plugin at Drive directly; the second run takes over the assets that are already in the catalog rather than creating a second copy.

## Access

The plugin reads Drive with a Google service account.

A service account on its own only sees files that have been **shared with its email address**, which is a reasonable way to catalogue a handful of shared folders. To read a whole organisation's Drive, give the service account [domain-wide delegation](https://developers.google.com/workspace/guides/create-credentials#optional_set_up_domain-wide_delegation_for_a_service_account) and set `impersonate_user` to a Workspace user to act as.

Enable the **Google Drive API**, and the **Google Sheets API** if you want the columns of each sheet. The scopes needed are:

```
https://www.googleapis.com/auth/drive.metadata.readonly
https://www.googleapis.com/auth/spreadsheets.readonly
```

Credentials follow Marmot's shared Google Cloud configuration: a key file, key JSON, or Application Default Credentials when nothing is set.

## Example Configuration

```yaml

credentials:
  credentials_file: "/etc/marmot/gcp-service-account.json"
impersonate_user: "data-platform@company.com"
tags:
  - "google-drive"

```

A single shared drive:

```yaml

credentials:
  credentials_file: "/etc/marmot/gcp-service-account.json"
drive_id: "0AItAbCdEfGhIjKlMnO"
exclude_mime_types:
  - "application/vnd.google-apps.shortcut"

```

## Configuration
The following configuration options are available:

| Property | Type | Required | Description |
|----------|------|----------|-------------|
| concurrency | int | false | Parallel requests for the sheets of a spreadsheet |
| credentials | GCPCredentials | false | GCP credentials configuration |
| drive_id | string | false | Shared drive to read. Empty means the user's My Drive |
| exclude_mime_types | []string | false | MIME types to skip, for example application/vnd.google-apps.shortcut |
| external_links | []ExternalLink | false | External links to show on all assets |
| filter | Filter | false | Filter discovered assets by name (regex) |
| folder_id | string | false | Only discover this folder and everything under it |
| header_row | int | false | Row of a sheet that holds the column names |
| impersonate_user | string | false | Workspace user to act as. Needed to read a whole organisation's Drive, and requires domain-wide delegation on the service account. Without it, only files shared with the service account are visible |
| include_files | bool | false | Discover files, not just folders |
| include_spreadsheets | bool | false | Discover Google Sheets, including each of their sheets |
| include_trashed | bool | false | Discover files in the trash |
| include_worksheets | bool | false | Discover each sheet of a spreadsheet as a table, with its columns. Needs the Sheets read scope |
| max_files | int | false | Stop after this many files. 0 means no limit |
| page_size | int | false | Files per API request |
| tags | TagsConfig | false | Tags to apply to discovered assets |

## Available Metadata

The following metadata fields are available:

| Field | Type | Description |
|-------|------|-------------|
| checksum | string | File checksum reported by Drive |
| column_count | int | Number of columns in a sheet |
| drive_id | string | Google Drive file id |
| file_extension | string | File extension |
| file_type | string | Broad kind of file, for example Document, Spreadsheet or Image |
| file_version | string | Drive version number |
| hidden | bool | Whether the sheet is hidden |
| mime_type | string | File MIME type |
| modified_at | string | When the item last changed in Drive |
| owners | []string | People who own the item in Drive |
| path | string | Location within the drive |
| row_count | int64 | Number of rows in a sheet |
| shared | bool | Whether the item is shared |
| size | int64 | Size in bytes |
| spreadsheet | string | Spreadsheet a sheet belongs to |
