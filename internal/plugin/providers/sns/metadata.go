package sns

// SNSFields represents SNS-specific metadata fields
// +marmot:metadata
type SNSFields struct {
	TopicArn               string            `json:"topic_arn" metadata:"topic_arn" description:"The ARN of the SNS topic"`
	Owner                  string            `json:"owner" metadata:"owner" description:"AWS account ID that owns the topic"`
	DisplayName            string            `json:"display_name" metadata:"display_name" description:"Display name of the topic"`
	Policy                 string            `json:"policy" metadata:"policy" description:"Access policy of the topic"`
	SubscriptionsPending   string            `json:"subscriptions_pending" metadata:"subscriptions_pending" description:"Number of pending subscriptions"`
	SubscriptionsConfirmed string            `json:"subscriptions_confirmed" metadata:"subscriptions_confirmed" description:"Number of confirmed subscriptions"`
	Tags                   map[string]string `json:"tags" metadata:"tags" description:"AWS resource tags"`
}
