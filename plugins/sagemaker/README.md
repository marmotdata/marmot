The SageMaker plugin discovers models, inference endpoints, model registry groups, feature groups and training jobs from Amazon SageMaker.

Models and model package groups are both catalogued as Models, told apart by the `kind` metadata field. A registry group named the same as a deployed model resolves to the same asset.

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

Edges into S3 and Glue name assets those plugins own. Marmot drops an edge whose other end is not catalogued.

## Required Permissions

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
