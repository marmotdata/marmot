The Firehose plugin discovers Amazon Data Firehose delivery streams across your AWS accounts. It captures each stream's configuration and destination settings, and links the stream to the Kinesis stream or Kafka topic it reads from and to the tables and buckets it writes to.

Firehose creates no assets for those systems. Every lineage edge points at an asset another Marmot plugin catalogs, so run the Kinesis, Kafka, S3, Glue, Redshift, OpenSearch, Snowflake or Iceberg plugin alongside this one to see the edges.

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
          "firehose:ListDeliveryStreams",
          "firehose:DescribeDeliveryStream",
          "firehose:ListTagsForDeliveryStream"
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
        Action: ["firehose:ListDeliveryStreams", "firehose:DescribeDeliveryStream"],
        Resource: "*"
      }
    ]
  }}
/>

## AWS Configuration

See [AWS Configuration](./Shared%20Configuration/AWS%20Configuration.md) for the supported AWS configuration options.
