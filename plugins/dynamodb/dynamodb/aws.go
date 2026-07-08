package dynamodb

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/credentials/stscreds"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	"sigs.k8s.io/yaml"
)

// AWSCredentials describes how the plugin authenticates against AWS.
type AWSCredentials struct {
	UseDefault     bool   `json:"use_default,omitempty" description:"Use AWS credentials from environment or default profile (recommended)" default:"true"`
	ID             string `json:"id,omitempty" description:"AWS access key ID"`
	Secret         string `json:"secret,omitempty" description:"AWS secret access key" sensitive:"true"`
	Token          string `json:"token,omitempty" description:"AWS session token" sensitive:"true"`
	Profile        string `json:"profile,omitempty" description:"AWS profile to use from shared credentials file"`
	Role           string `json:"role,omitempty" description:"AWS IAM role ARN to assume"`
	RoleExternalID string `json:"role_external_id,omitempty" description:"External ID for cross-account role assumption"`
	Region         string `json:"region,omitempty" description:"AWS region for services"`
	Endpoint       string `json:"endpoint,omitempty" description:"Custom endpoint URL for AWS services" validate:"omitempty,url"`
}

// AWSConfig groups AWS-specific configuration common to all AWS-backed plugins.
type AWSConfig struct {
	Credentials    AWSCredentials `json:"credentials" description:"AWS credentials configuration"`
	TagsToMetadata bool           `json:"tags_to_metadata,omitempty" description:"Convert AWS tags to Marmot metadata"`
	IncludeTags    []string       `json:"include_tags,omitempty" description:"List of AWS tags to include as metadata. By default, all tags are included."`
}

func extractAWSConfig(raw map[string]interface{}) (*AWSConfig, error) {
	var cfg AWSConfig
	b, err := yaml.Marshal(raw)
	if err != nil {
		return nil, fmt.Errorf("marshaling raw config: %w", err)
	}
	if err := yaml.Unmarshal(b, &cfg); err != nil {
		return nil, fmt.Errorf("unmarshaling into AWSConfig: %w", err)
	}
	return &cfg, nil
}

func (a *AWSConfig) newAWSConfig(ctx context.Context) (aws.Config, error) {
	var opts []func(*awsconfig.LoadOptions) error

	if a.Credentials.Region != "" {
		opts = append(opts, awsconfig.WithRegion(a.Credentials.Region))
	}

	if a.Credentials.UseDefault || (a.Credentials.ID == "" && a.Credentials.Profile == "") {
		if a.Credentials.Profile != "" {
			opts = append(opts, awsconfig.WithSharedConfigProfile(a.Credentials.Profile))
		}
	} else {
		if a.Credentials.ID != "" && a.Credentials.Secret != "" {
			provider := credentials.NewStaticCredentialsProvider(
				a.Credentials.ID,
				a.Credentials.Secret,
				a.Credentials.Token,
			)
			opts = append(opts, awsconfig.WithCredentialsProvider(provider))
		}

		if a.Credentials.Profile != "" {
			opts = append(opts, awsconfig.WithSharedConfigProfile(a.Credentials.Profile))
		}
	}

	cfg, err := awsconfig.LoadDefaultConfig(ctx, opts...)
	if err != nil {
		return aws.Config{}, fmt.Errorf("loading AWS config: %w", err)
	}

	if a.Credentials.Role != "" {
		stsClient := sts.NewFromConfig(cfg)
		assumeRoleOpts := func(o *stscreds.AssumeRoleOptions) {
			if a.Credentials.RoleExternalID != "" {
				o.ExternalID = aws.String(a.Credentials.RoleExternalID)
			}
		}

		provider := stscreds.NewAssumeRoleProvider(stsClient, a.Credentials.Role, assumeRoleOpts)
		cfg.Credentials = aws.NewCredentialsCache(provider)
	}

	if a.Credentials.Endpoint != "" {
		cfg.BaseEndpoint = aws.String(a.Credentials.Endpoint)
	}

	return cfg, nil
}

func processAWSTags(tagsToMetadata bool, includeTags []string, tags map[string]string) map[string]interface{} {
	metadata := make(map[string]interface{})

	if !tagsToMetadata || len(tags) == 0 {
		return metadata
	}

	for key, value := range tags {
		if len(includeTags) > 0 {
			included := false
			for _, includeTag := range includeTags {
				if key == includeTag {
					included = true
					break
				}
			}
			if !included {
				continue
			}
		}

		metadata[fmt.Sprintf("tag_%s", key)] = value
	}

	return metadata
}
