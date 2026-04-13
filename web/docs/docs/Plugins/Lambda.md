---
title: Lambda
description: This plugin discovers Lambda functions from AWS accounts.
status: experimental
---

# Lambda

<div class="flex flex-col gap-3 mb-6 pb-6 border-b border-gray-200">
<div class="flex items-center gap-3">
<span class="inline-flex items-center rounded-full px-4 py-2 text-sm font-medium bg-earthy-yellow-300 text-earthy-yellow-900">Experimental</span>
</div>
<div class="flex items-center gap-2">
<span class="text-sm text-gray-500">Creates:</span>
<div class="flex flex-wrap gap-2"><span class="inline-flex items-center rounded-lg px-4 py-2 text-sm font-medium bg-earthy-green-100 text-earthy-green-800 border border-earthy-green-300">Assets</span></div>
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


The Lambda plugin discovers and catalogs AWS Lambda functions across your AWS accounts. It captures function metadata including runtime, memory, timeout, VPC configuration, layers, tracing, and tags.

## Required Permissions

import { Collapsible } from "@site/src/components/Collapsible";

<Collapsible
  title="IAM Policy"
  icon="mdi:shield-check"
  policyJson={{
    Version: "2012-10-17",
    Statement: [
      {
        Effect: "Allow",
        Action: [
          "lambda:ListFunctions",
          "lambda:GetFunction",
          "lambda:ListTags"
        ],
        Resource: "*"
      }
    ]
  }}
  minimalPolicyJson={{
    Version: "2012-10-17",
    Statement: [
      {
        Effect: "Allow",
        Action: ["lambda:ListFunctions"],
        Resource: "*"
      }
    ]
  }}
/>

## AWS Configuration

See [AWS Configuration](./Shared%20Configuration/AWS%20Configuration.md) for the supported AWS configuration options.



## Example Configuration

```yaml

credentials:
  region: "us-east-1"
  profile: "production"
  role: "<role>"
tags:
  - "aws"

```

## Configuration
The following configuration options are available:

| Property | Type | Required | Description |
|----------|------|----------|-------------|
| credentials | AWSCredentials | false | AWS credentials configuration |
| external_links | []ExternalLink | false | External links to show on all assets |
| filter | Filter | false | Filter discovered assets by name (regex) |
| include_tags | []string | false | List of AWS tags to include as metadata. By default, all tags are included. |
| tags | TagsConfig | false | Tags to apply to discovered assets |
| tags_to_metadata | bool | false | Convert AWS tags to Marmot metadata |

## Available Metadata

The following metadata fields are available:

| Field | Type | Description |
|-------|------|-------------|
| architectures | string | Instruction set architectures (x86_64, arm64) |
| code_sha256 | string | SHA256 hash of the deployment package |
| code_size | int64 | The size of the function's deployment package in bytes |
| description | string | The function's description |
| environment_variable_count | int | Number of environment variables configured |
| ephemeral_storage_mb | int32 | Ephemeral storage allocated in MB |
| function_arn | string | The ARN of the Lambda function |
| handler | string | The function's entry point handler |
| last_modified | string | Date and time the function was last modified |
| last_update_status | string | Status of the last update (Successful, Failed, InProgress) |
| layer_count | int | Number of Lambda layers attached |
| layers | string | Lambda layer ARNs attached to the function |
| memory_size_mb | int32 | Memory allocated to the function in MB |
| package_type | string | Deployment package type (Zip or Image) |
| role | string | The IAM execution role ARN |
| runtime | string | The runtime environment for the function (e.g. go1.x, python3.12, nodejs20.x) |
| security_group_count | int | Number of VPC security groups |
| state | string | Current state of the function (Active, Pending, Inactive, Failed) |
| subnet_count | int | Number of VPC subnets |
| tags | map[string]string | AWS resource tags |
| timeout_seconds | int32 | Function execution timeout in seconds |
| tracing_mode | string | X-Ray tracing mode (Active or PassThrough) |
| version | string | The function version |
| vpc_id | string | VPC ID if the function is connected to a VPC |