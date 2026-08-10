The Google Drive plugin catalogues the documents and sheets a team keeps in Drive, so the spreadsheet a report is actually built from is findable next to the warehouse tables feeding it.

A drive is imported as a tree: the drive itself at the root, folders nested under it, files linked to the folder holding them. Google Sheets can be imported as spreadsheets, with each sheet under them optionally imported as a table whose header row becomes the columns.

These are the same asset types and names the [OpenMetadata](./OpenMetadata) plugin produces for a Google Drive service, so an organisation moving off OpenMetadata can import their drive from there and later point this plugin at Drive directly; the second run takes over the assets already in the catalog rather than creating a second copy.

## Access

The plugin reads Drive with a Google service account.

A service account on its own only sees files that have been **shared with its email address**, which is a reasonable way to catalogue a handful of shared folders. To read a whole organisation's Drive, give the service account [domain-wide delegation](https://developers.google.com/workspace/guides/create-credentials#optional_set_up_domain-wide_delegation_for_a_service_account) and set `impersonate_user` to a Workspace user to act as.

Enable the **Google Drive API**, and the **Google Sheets API** if you want the columns of each sheet. The scopes needed are:

```
https://www.googleapis.com/auth/drive.metadata.readonly
https://www.googleapis.com/auth/spreadsheets.readonly
```

Credentials follow Marmot's shared Google Cloud configuration: a key file, key JSON, or Application Default Credentials when nothing is set.
