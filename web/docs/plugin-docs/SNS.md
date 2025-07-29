The SNS plugin discovers and catalogs Amazon SNS topics across your AWS accounts. It captures topic configurations, subscription details, access policies, and AWS resource tags.

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
        "sns:ListTopics",
        "sns:GetTopicAttributes",
        "sns:ListTagsForResource"
      ],
      "Resource": "*"
    }
  ]
}
```

### Minimal Permissions

For basic topic discovery without tags:

```json
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Action": ["sns:ListTopics", "sns:GetTopicAttributes"],
      "Resource": "*"
    }
  ]
}
```
