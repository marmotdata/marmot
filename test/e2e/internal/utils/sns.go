package utils

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/sns"
	"github.com/aws/aws-sdk-go-v2/service/sns/types"
)

type TestTopic struct {
	Name string
	Tags map[string]string
}

func CreateTestTopics(ctx context.Context, topics []TestTopic) error {
	customEndpoint := "http://localhost:4566"
	cfg, err := config.LoadDefaultConfig(ctx,
		config.WithRegion("us-east-1"),
		config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider("test", "test", "")),
	)
	if err != nil {
		return fmt.Errorf("configuring AWS SDK: %w", err)
	}

	client := sns.NewFromConfig(cfg, func(o *sns.Options) {
		o.BaseEndpoint = aws.String(customEndpoint)
	})

	for _, topic := range topics {
		input := &sns.CreateTopicInput{
			Name: aws.String(topic.Name),
			Tags: make([]types.Tag, 0, len(topic.Tags)),
		}

		for k, v := range topic.Tags {
			input.Tags = append(input.Tags, types.Tag{
				Key:   aws.String(k),
				Value: aws.String(v),
			})
		}

		_, err := client.CreateTopic(ctx, input)
		if err != nil {
			return fmt.Errorf("creating topic %s: %w", topic.Name, err)
		}
	}

	return nil
}
