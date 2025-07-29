The SQS plugin discovers and catalogs Amazon SQS queues across your AWS accounts. It captures queue configurations, attributes, and can optionally discover Dead Letter Queue relationships between queues.

## Prerequisites

### AWS Permissions

The plugin requires the following IAM permissions:

```json
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Action": [
        "sqs:ListQueues",
        "sqs:GetQueueAttributes",
        "sqs:ListQueueTags"
      ],
      "Resource": "*"
    }
  ]
}
```

### Minimal Permissions

For basic queue discovery without tags:

```json
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Action": ["sqs:ListQueues", "sqs:GetQueueAttributes"],
      "Resource": "*"
    }
  ]
}
```
