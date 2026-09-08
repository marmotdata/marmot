The Kinesis plugin discovers Amazon Kinesis Data Streams across your AWS accounts. Each stream becomes a Stream asset carrying its capacity mode, retention, shard and consumer counts, encryption settings and tags, with a data preview that reads the oldest retained records from the first open shard.

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
          "kinesis:ListStreams",
          "kinesis:DescribeStreamSummary",
          "kinesis:ListShards",
          "kinesis:ListStreamConsumers",
          "kinesis:ListTagsForStream",
          "kinesis:GetShardIterator",
          "kinesis:GetRecords"
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
        Action: ["kinesis:ListStreams", "kinesis:DescribeStreamSummary"],
        Resource: "*"
      }
    ]
  }}
/>

`GetShardIterator` and `GetRecords` are only needed for the data preview. `ListShards`, `ListStreamConsumers` and `ListTagsForStream` can be left out when `include_shards`, `include_consumers` or `tags_to_metadata` are switched off; a denied call logs a warning and the stream is still catalogued.

## AWS Configuration

See [AWS Configuration](./Shared%20Configuration/AWS%20Configuration.md) for the supported AWS configuration options.
