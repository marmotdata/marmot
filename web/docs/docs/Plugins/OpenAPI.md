---
title: OpenAPI
description: This plugin discovers OpenAPI v3 specifications.
status: experimental
---

# OpenAPI

<div class="flex flex-col gap-3 mb-6 pb-6 border-b border-gray-200">
<div class="flex items-center gap-3">
<span class="inline-flex items-center rounded-full px-4 py-2 text-sm font-medium bg-earthy-yellow-300 text-earthy-yellow-900">Experimental</span>
</div>
<div class="flex items-center gap-2">
<span class="text-sm text-gray-500">Creates:</span>
<div class="flex flex-wrap gap-2"><span class="inline-flex items-center rounded-lg px-4 py-2 text-sm font-medium bg-earthy-green-100 text-earthy-green-800 border border-earthy-green-300">Assets</span></div>
</div>
</div>


The OpenAPI plugin discovers API specifications from OpenAPI v3 files. It creates assets for services and their endpoints.

The plugin scans for `.json` and `.yaml` files and parses them as OpenAPI v3 specifications.


## Example Configuration

```yaml

spec_path: "/app/openapi-specs"
tags:
  - "openapi"
  - "specifications"

```

## Configuration
The following configuration options are available:

| Property | Type | Required | Description |
|----------|------|----------|-------------|
| external_links | []ExternalLink | false | External links to show on all assets |
| spec_path | string | false | Path to the directory containing the OpenAPI specifications |
| tags | TagsConfig | false | Tags to apply to discovered assets |

## Available Metadata

The following metadata fields are available:

| Field | Type | Description |
|-------|------|-------------|
| contact_email | string | Contact email |
| contact_name | string | Contact name |
| contact_url | string | Contact URL |
| deprecated | bool | Is this endpoint deprecated |
| description | string | Description of the API |
| description | string | A verbose explanation of the operation behaviour. |
| external_docs | string | Link to the external documentation |
| http_method | string | HTTP method |
| license_identifier | string | SPDX license experession for the API |
| license_name | string | Name of the license |
| license_url | string | URL of the license |
| num_deprecated_endpoints | int | Number of deprecated endpoints in the OpenAPI specification |
| num_endpoints | int | Number of endpoints in the OpenAPI specification |
| openapi_version | string | Version of the OpenAPI spec |
| operation_id | string | Unique identifier of the operation |
| path | string | Path |
| servers | []string | URL of the servers of the API |
| service_name | string | Name of the service that owns the resource |
| service_version | string | Version of the service |
| status_codes | []string | All HTTP response status codes that are returned for this endpoint. |
| summary | string | A short summary of what the operation does |
| terms_of_service | string | Link to the page that describes the terms of service |