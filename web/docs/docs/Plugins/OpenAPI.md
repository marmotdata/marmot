---
title: OpenAPI
description: This plugin discovers OpenAPI v3.0 specifications.
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
| include_openapi_tags | bool | false | Inlcude tags from OpenAPI specification |
| merge | MergeConfig | false |  |
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
| description | string | Description of the API |
| external_docs | string | Link to the external documentation |
| http_method | string | HTTP method |
| license_identifier | string | SPDX license experession for the API |
| license_name | string | Name of the license |
| license_url | string | URL of the license |
| num_endpoints | int | Number of endpoints in the OpenAPI specification |
| openapi_version | string | Version of the OpenAPI spec |
| path | string | Path |
| servers | []string | URL of the servers of the API |
| service_name | string | Name of the service that owns the resource |
| service_version | string | Version of the service |
| terms_of_service | string | Link to the page that describes the terms of service |