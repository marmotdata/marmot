The Iceberg plugin discovers namespaces, tables and views from Iceberg catalogs. It supports both REST catalogs and AWS Glue Data Catalog as backends.

## AWS Glue Catalog Permissions

When using `catalog_type: "glue"`, the following IAM permissions are required:

<Collapsible
  title="IAM Policy"
  icon="mdi:shield-check"
  policyJson={{
    Version: "2012-10-17",
    Statement: [
      {
        Effect: "Allow",
        Action: [
          "glue:GetDatabases",
          "glue:GetDatabase",
          "glue:GetTables",
          "glue:GetTable"
        ],
        Resource: "*"
      },
      {
        Effect: "Allow",
        Action: [
          "s3:GetObject"
        ],
        Resource: "arn:aws:s3:::*/*",
        Condition: {
          StringLike: {
            "s3:prefix": "*/metadata/*"
          }
        }
      }
    ]
  }}
  minimalPolicyJson={{
    Version: "2012-10-17",
    Statement: [
      {
        Effect: "Allow",
        Action: [
          "glue:GetDatabases",
          "glue:GetTables",
          "glue:GetTable"
        ],
        Resource: "*"
      },
      {
        Effect: "Allow",
        Action: [
          "s3:GetObject"
        ],
        Resource: "arn:aws:s3:::*/*"
      }
    ]
  }}
/>

The `s3:GetObject` permission is needed because Glue's `LoadTable` reads Iceberg metadata files from S3.

## AWS Configuration

When using `catalog_type: "glue"`, see [AWS Configuration](./Shared%20Configuration/AWS%20Configuration.md) for the supported AWS configuration options.

## Keyless authentication

On Marmot Cloud or Marmot Enterprise the pipeline can present its own identity instead of access keys. Register your Marmot instance as an IAM OIDC identity provider (client id `sts.amazonaws.com`), create a role whose trust policy allows `sts:AssumeRoleWithWebIdentity` for that provider with `<issuer host>:sub` equal to the pipeline's subject, `pipeline:<name>` as reported by the pipeline API, and set `credentials.role_arn` and `credentials.region`. No key exists anywhere; Marmot mints a short-lived token for each run.
