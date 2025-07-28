package utils

import (
	"context"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
)

type TestBucket struct {
	Name                string
	Tags                map[string]string
	EnableVersioning    bool
	EnableEncryption    bool
	EnableWebsite       bool
	EnableLogging       bool
	EnableAccelerate    bool
	EnableLifecycle     bool
	EnableReplication   bool
	EnableNotifications bool
	EnablePublicAccess  bool
	RequestPayerConfig  string
}

func CreateTestBuckets(ctx context.Context, buckets []TestBucket) error {
	customEndpoint := "http://localhost:4566"
	cfg, err := config.LoadDefaultConfig(ctx,
		config.WithRegion("us-east-1"),
		config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider("test", "test", "")),
	)
	if err != nil {
		return fmt.Errorf("configuring AWS SDK: %w", err)
	}

	client := s3.NewFromConfig(cfg, func(o *s3.Options) {
		o.BaseEndpoint = aws.String(customEndpoint)
		o.UsePathStyle = true
	})

	for _, bucket := range buckets {
		_, err := client.CreateBucket(ctx, &s3.CreateBucketInput{
			Bucket: aws.String(bucket.Name),
		})
		if err != nil {
			return fmt.Errorf("creating bucket %s: %w", bucket.Name, err)
		}

		if len(bucket.Tags) > 0 {
			var tagSet []types.Tag
			for k, v := range bucket.Tags {
				tagSet = append(tagSet, types.Tag{
					Key:   aws.String(k),
					Value: aws.String(v),
				})
			}

			_, err = client.PutBucketTagging(ctx, &s3.PutBucketTaggingInput{
				Bucket: aws.String(bucket.Name),
				Tagging: &types.Tagging{
					TagSet: tagSet,
				},
			})
			if err != nil {
				return fmt.Errorf("setting tags for bucket %s: %w", bucket.Name, err)
			}
		}

		if bucket.EnableVersioning {
			_, err = client.PutBucketVersioning(ctx, &s3.PutBucketVersioningInput{
				Bucket: aws.String(bucket.Name),
				VersioningConfiguration: &types.VersioningConfiguration{
					Status: types.BucketVersioningStatusEnabled,
				},
			})
			if err != nil {
				return fmt.Errorf("enabling versioning for bucket %s: %w", bucket.Name, err)
			}
		}

		if bucket.EnableEncryption {
			_, err = client.PutBucketEncryption(ctx, &s3.PutBucketEncryptionInput{
				Bucket: aws.String(bucket.Name),
				ServerSideEncryptionConfiguration: &types.ServerSideEncryptionConfiguration{
					Rules: []types.ServerSideEncryptionRule{
						{
							ApplyServerSideEncryptionByDefault: &types.ServerSideEncryptionByDefault{
								SSEAlgorithm: types.ServerSideEncryptionAes256,
							},
						},
					},
				},
			})
			if err != nil {
				return fmt.Errorf("enabling encryption for bucket %s: %w", bucket.Name, err)
			}
		}

		if bucket.EnableWebsite {
			_, err = client.PutBucketWebsite(ctx, &s3.PutBucketWebsiteInput{
				Bucket: aws.String(bucket.Name),
				WebsiteConfiguration: &types.WebsiteConfiguration{
					IndexDocument: &types.IndexDocument{
						Suffix: aws.String("index.html"),
					},
					ErrorDocument: &types.ErrorDocument{
						Key: aws.String("error.html"),
					},
				},
			})
			if err != nil {
				return fmt.Errorf("enabling website hosting for bucket %s: %w", bucket.Name, err)
			}
		}

		if bucket.EnableLogging {
			_, err = client.PutBucketLogging(ctx, &s3.PutBucketLoggingInput{
				Bucket: aws.String(bucket.Name),
				BucketLoggingStatus: &types.BucketLoggingStatus{
					LoggingEnabled: &types.LoggingEnabled{
						TargetBucket: aws.String(bucket.Name),
						TargetPrefix: aws.String("logs/"),
					},
				},
			})
			if err != nil {
				return fmt.Errorf("enabling logging for bucket %s: %w", bucket.Name, err)
			}
		}

		if bucket.EnableAccelerate {
			_, err = client.PutBucketAccelerateConfiguration(ctx, &s3.PutBucketAccelerateConfigurationInput{
				Bucket: aws.String(bucket.Name),
				AccelerateConfiguration: &types.AccelerateConfiguration{
					Status: types.BucketAccelerateStatusEnabled,
				},
			})
			if err != nil {
				return fmt.Errorf("enabling acceleration for bucket %s: %w", bucket.Name, err)
			}
		}

		if bucket.EnableLifecycle {
			_, err = client.PutBucketLifecycleConfiguration(ctx, &s3.PutBucketLifecycleConfigurationInput{
				Bucket: aws.String(bucket.Name),
				LifecycleConfiguration: &types.BucketLifecycleConfiguration{
					Rules: []types.LifecycleRule{
						{
							ID:     aws.String("test-rule"),
							Status: types.ExpirationStatusEnabled,
							Filter: &types.LifecycleRuleFilter{
								Prefix: aws.String("temp/"),
							},
							Expiration: &types.LifecycleExpiration{
								Days: aws.Int32(30),
							},
						},
					},
				},
			})
			if err != nil {
				return fmt.Errorf("enabling lifecycle for bucket %s: %w", bucket.Name, err)
			}
		}

		if bucket.EnableNotifications {
			_, err = client.PutBucketNotificationConfiguration(ctx, &s3.PutBucketNotificationConfigurationInput{
				Bucket: aws.String(bucket.Name),
				NotificationConfiguration: &types.NotificationConfiguration{
					TopicConfigurations: []types.TopicConfiguration{
						{
							Id:       aws.String("test-topic-config"),
							TopicArn: aws.String("arn:aws:sns:us-east-1:123456789012:test-topic"),
							Events: []types.Event{
								types.EventS3ObjectCreated,
							},
						},
					},
				},
			})
			if err != nil && !strings.Contains(err.Error(), "InvalidArgument") {
				return fmt.Errorf("enabling notifications for bucket %s: %w", bucket.Name, err)
			}
		}

		if !bucket.EnablePublicAccess {
			_, err = client.PutPublicAccessBlock(ctx, &s3.PutPublicAccessBlockInput{
				Bucket: aws.String(bucket.Name),
				PublicAccessBlockConfiguration: &types.PublicAccessBlockConfiguration{
					BlockPublicAcls:       aws.Bool(true),
					BlockPublicPolicy:     aws.Bool(true),
					IgnorePublicAcls:      aws.Bool(true),
					RestrictPublicBuckets: aws.Bool(true),
				},
			})
			if err != nil {
				return fmt.Errorf("setting public access block for bucket %s: %w", bucket.Name, err)
			}
		}

		if bucket.RequestPayerConfig != "" {
			var payer types.Payer
			if bucket.RequestPayerConfig == "Requester" {
				payer = types.PayerRequester
			} else {
				payer = types.PayerBucketOwner
			}

			_, err = client.PutBucketRequestPayment(ctx, &s3.PutBucketRequestPaymentInput{
				Bucket: aws.String(bucket.Name),
				RequestPaymentConfiguration: &types.RequestPaymentConfiguration{
					Payer: payer,
				},
			})
			if err != nil {
				return fmt.Errorf("setting request payment for bucket %s: %w", bucket.Name, err)
			}
		}
	}

	return nil
}
