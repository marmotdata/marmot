---
title: OpenAPI
description: This plugin discovers OpenAPI v3 specifications.
status: experimental
---

# OpenAPI

**Status:** experimental

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
| aws | AWSConfig | false |  |
| external_links | []ExternalLink | false |  |
| global_documentation | []string | false |  |
| global_documentation_position | string | false |  |
| metadata | MetadataConfig | false |  |
| spec_path | string | false | Path to the directory containing the OpenAPI specifications |
| tags | TagsConfig | false |  |

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
| license_identifier | string | SPDX licence expression for the API |
| license_name | string | Name of the licence |
| license_url | string | URL of the licence |
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