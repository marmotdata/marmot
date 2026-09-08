---
title: SageMaker
description: Discovers models, endpoints, feature groups and training jobs from Amazon SageMaker.
status: experimental
---

# SageMaker

<div class="flex flex-col gap-3 mb-6 pb-6 border-b border-gray-200">
<div class="flex items-center gap-3">
<span class="inline-flex items-center rounded-full px-4 py-2 text-sm font-medium bg-earthy-yellow-300 text-earthy-yellow-900">Experimental</span>
</div>
<div class="flex items-center gap-2">
<span class="text-sm text-gray-500">Creates:</span>
<div class="flex flex-wrap gap-2"><span class="inline-flex items-center rounded-lg px-4 py-2 text-sm font-medium bg-earthy-green-100 text-earthy-green-800 border border-earthy-green-300">Assets</span><span class="inline-flex items-center rounded-lg px-4 py-2 text-sm font-medium bg-earthy-green-100 text-earthy-green-800 border border-earthy-green-300">Lineage</span><span class="inline-flex items-center rounded-lg px-4 py-2 text-sm font-medium bg-earthy-green-100 text-earthy-green-800 border border-earthy-green-300">Run History</span></div>
</div>
</div>

import { CalloutCard } from '@site/src/components/DocCard';

<CalloutCard
  title="Configure in the UI"
  description="This plugin can be configured directly in the Marmot UI with a step-by-step wizard."
  docId="Populating/UI"
  buttonText="View Guide"
  variant="secondary"
  icon="mdi:cursor-default-click"
/>


The SageMaker plugin discovers models, inference endpoints, model registry groups, feature groups and training jobs from Amazon SageMaker.

Models and model package groups are both catalogued as Models, told apart by the `kind` metadata field. A registry group named the same as a deployed model resolves to the same asset. Feature groups are catalogued as Datasets, with their feature definitions as the schema, and training jobs as Jobs with run history.

Training jobs are off by default because an account keeps a long job history. Turn them on with `include_training_jobs: true`.

## Lineage

| Edge | Meaning |
|------|---------|
| S3 bucket FEEDS model | The bucket holding the model artifact |
| Model FEEDS endpoint | A production variant serves the model |
| S3 bucket FEEDS training job | A training channel reads from the bucket |
| Training job PRODUCES model | The model's artifact is the one the job wrote |
| Feature group PRODUCES Glue table | The offline store is queryable through the table |
| Feature group PRODUCES S3 bucket | The offline store writes to the bucket |

Edges into S3 and Glue name assets those plugins own. Marmot drops an edge whose other end is not catalogued, so run the S3 and Glue plugins alongside this one to see them.

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
          "sagemaker:ListModels",
          "sagemaker:DescribeModel",
          "sagemaker:ListEndpoints",
          "sagemaker:DescribeEndpoint",
          "sagemaker:DescribeEndpointConfig",
          "sagemaker:ListModelPackageGroups",
          "sagemaker:DescribeModelPackageGroup",
          "sagemaker:ListModelPackages",
          "sagemaker:DescribeModelPackage",
          "sagemaker:ListFeatureGroups",
          "sagemaker:DescribeFeatureGroup",
          "sagemaker:ListTrainingJobs",
          "sagemaker:DescribeTrainingJob",
          "sagemaker:ListTags"
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
        Action: ["sagemaker:ListModels", "sagemaker:DescribeModel"],
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
tags_to_metadata: true
include_endpoints: true
include_model_packages: true
include_feature_groups: true
include_training_jobs: false
tags:
  - "aws"
  - "ml"

```

## Configuration
The following configuration options are available:

| Property | Type | Required | Description |
|----------|------|----------|-------------|
| credentials | AWSCredentials | false | AWS credentials configuration |
| external_links | []ExternalLink | false | External links to show on all assets |
| filter | Filter | false | Filter discovered assets by name (regex) |
| include_endpoints | bool | false | Whether to discover inference endpoints |
| include_feature_groups | bool | false | Whether to discover feature groups from the feature store |
| include_model_packages | bool | false | Whether to discover model package groups from the model registry |
| include_tags | []string | false | List of AWS tags to include as metadata. By default, all tags are included. |
| include_training_jobs | bool | false | Whether to discover training jobs. Accounts keep a long job history, so this is off by default |
| tags | TagsConfig | false | Tags to apply to discovered assets |
| tags_to_metadata | bool | false | Convert AWS tags to Marmot metadata |

## Available Metadata

The following metadata fields are available:

| Field | Type | Description |
|-------|------|-------------|
| algorithm_image | string | Container image the job trained with |
| algorithm_name | string | Marketplace algorithm the job trained with |
| arn | string | The ARN of the model, endpoint, feature group or training job |
| column_name | string | Feature name |
| containers | []object | Containers of an inference pipeline, each with its image, model_data_url and mode |
| created_at | string | When the resource was created |
| data_capture_s3_uri | string | S3 location captured requests and responses are written to |
| data_type | string | Feature type (String, Integral, Fractional) |
| description | string | Description of the model package group or feature group |
| domain | string | Machine learning domain the model package belongs to |
| ended_at | string | When training ended |
| endpoint_config | string | Name of the endpoint configuration in use |
| environment | map[string]string | Container environment variables, with credential-looking values masked |
| event_time_feature | string | Feature holding the event timestamp |
| execution_role_arn | string | IAM role the model runs under |
| failure_reason | string | Why the training job failed |
| final_metrics | map[string]float32 | Last value the job reported for each metric |
| glue_table | string | Glue table the offline store is queryable through, as database.table |
| hyperparameters | map[string]string | Hyperparameters the job ran with |
| image | string | Container image the model is served from |
| input_channels | map[string]string | Training channels, each mapping a channel name to its S3 location |
| instance_count | int32 | Number of instances the job ran on |
| instance_type | string | Instance type the job ran on |
| is_nullable | bool | False for the record identifier and event time features, which every record must carry |
| is_primary_key | bool | Whether the feature is the record identifier |
| kind | string | Which kind of model this is (model, model_package_group) |
| kms_key_id | string | KMS key the endpoint storage volume is encrypted with |
| last_modified_at | string | When the endpoint was last modified |
| latest_approval_status | string | Approval status of the newest version |
| latest_image | string | Container image of the newest approved version |
| latest_model_data_url | string | S3 artifact of the newest approved version |
| latest_version | int32 | Newest version number in the model package group |
| mode | string | Container mode (SingleModel or MultiModel) |
| model_artifacts_s3_uri | string | S3 location of the model artifact the job produced |
| model_data_url | string | S3 location of the model artifact |
| model_quality_statistics_s3_uri | string | S3 location of the model quality statistics report |
| network_isolation | bool | Whether the model container runs without network access |
| offline_store_s3_uri | string | S3 location of the offline store |
| online_store | bool | Whether the online store is enabled |
| output_s3_path | string | S3 prefix the job wrote its output to |
| record_identifier | string | Feature that identifies a record |
| region | string | AWS region the resource lives in |
| sample_payload_url | string | S3 location of a sample inference payload |
| started_at | string | When training started |
| status | string | Status of the resource |
| supported_content_types | []string | Content types the model package accepts |
| supported_response_mime_types | []string | Response MIME types the model package returns |
| task | string | Machine learning task the model package performs |
| url | string | Link to the resource in the AWS console. Not set for model package groups or feature groups, which the classic console has no route to |
| variants | []object | Production variants, each with its name, model, instance_type, instance_count, weight and serverless flag |
| version_count | int | Number of versions in the model package group |
| vpc_subnet_count | int | Number of VPC subnets the model is attached to |

AWS resource tags are added as `tag_<key>` when `tags_to_metadata` is on.
