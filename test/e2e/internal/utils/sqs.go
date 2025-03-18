package utils

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
)

type TestQueue struct {
	Name string
	Tags map[string]string
}

func CreateTestQueues(ctx context.Context, queues []TestQueue) error {
	customEndpoint := "http://localhost:4566"
	cfg, err := config.LoadDefaultConfig(ctx,
		config.WithRegion("us-east-1"),
		config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider("test", "test", "")),
	)
	if err != nil {
		return fmt.Errorf("configuring AWS SDK: %w", err)
	}

	client := sqs.NewFromConfig(cfg, func(o *sqs.Options) {
		o.BaseEndpoint = aws.String(customEndpoint)
	})

	for _, queue := range queues {
		input := &sqs.CreateQueueInput{
			QueueName: aws.String(queue.Name),
			Tags:      queue.Tags,
		}

		_, err := client.CreateQueue(ctx, input)
		if err != nil {
			return fmt.Errorf("creating queue %s: %w", queue.Name, err)
		}
	}

	return nil
}
