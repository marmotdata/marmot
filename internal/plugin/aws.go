package plugin

import (
	"context"
	"fmt"
	"regexp"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/credentials/stscreds"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	"sigs.k8s.io/yaml"
)

// AWSCredentials represents AWS authentication configuration
type AWSCredentials struct {
	Profile        string `json:"profile,omitempty" description:"AWS profile to use from shared credentials file"`
	ID             string `json:"id,omitempty" description:"AWS access key ID"`
	Secret         string `json:"secret,omitempty" description:"AWS secret access key"`
	Endpoint       string `json:"endpoint,omitempty" description:"AWS endpoint"`
	Token          string `json:"token,omitempty" description:"AWS session token"`
	Role           string `json:"role,omitempty" description:"AWS IAM role ARN to assume"`
	RoleExternalID string `json:"role_external_id,omitempty" description:"External ID for cross-account role assumption"`
	Region         string `json:"region" description:"AWS region for services" required:"true"`
}

// Filter represents include/exclude patterns for AWS resources
type Filter struct {
	Include []string `json:"include,omitempty" description:"Include patterns for resource names (regex)"`
	Exclude []string `json:"exclude,omitempty" description:"Exclude patterns for resource names (regex)"`
}

// AWSConfig represents common AWS configuration for plugins
type AWSConfig struct {
	Credentials    AWSCredentials `json:"credentials" description:"AWS credentials configuration"`
	TagsToMetadata bool           `json:"tags_to_metadata,omitempty" description:"Convert AWS tags to Marmot metadata"`
	IncludeTags    []string       `json:"include_tags,omitempty" description:"List of AWS tags to include as metadata"`
	Filter         Filter         `json:"filter,omitempty" description:"Filter patterns for AWS resources"`
}

// Validate validates the AWSConfig
func (a *AWSConfig) Validate() error {
	if a.Credentials.Region == "" {
		return fmt.Errorf("AWS region is required")
	}
	return nil
}

type AWSPlugin struct {
	AWSConfig  `json:",inline"`
	BaseConfig `json:",inline"`
}

// NewAWSConfig loads AWS configuration with the provided credentials
func NewAWSConfig(ctx context.Context, rawConfig map[string]interface{}) (aws.Config, error) {
	// Directly unmarshal into AWSConfig
	var awsCfg AWSConfig
	configBytes, err := yaml.Marshal(rawConfig)
	if err != nil {
		return aws.Config{}, fmt.Errorf("marshaling raw config: %w", err)
	}
	if err := yaml.Unmarshal(configBytes, &awsCfg); err != nil {
		return aws.Config{}, fmt.Errorf("unmarshaling into AWSConfig: %w", err)
	}

	var opts []func(*config.LoadOptions) error

	// Always set the region
	opts = append(opts, config.WithRegion(awsCfg.Credentials.Region))

	// Handle custom endpoint
	if awsCfg.Credentials.Endpoint != "" {
		customResolver := aws.EndpointResolverWithOptionsFunc(func(service, region string, options ...interface{}) (aws.Endpoint, error) {
			return aws.Endpoint{
				URL: awsCfg.Credentials.Endpoint,
			}, nil
		})
		opts = append(opts, config.WithEndpointResolverWithOptions(customResolver))
	}

	// Handle static credentials
	if awsCfg.Credentials.ID != "" && awsCfg.Credentials.Secret != "" {
		provider := credentials.NewStaticCredentialsProvider(awsCfg.Credentials.ID, awsCfg.Credentials.Secret, awsCfg.Credentials.Token)
		opts = append(opts, config.WithCredentialsProvider(provider))
	}

	// Handle profile
	if awsCfg.Credentials.Profile != "" {
		opts = append(opts, config.WithSharedConfigProfile(awsCfg.Credentials.Profile))
	}

	// Load the configuration
	cfg, err := config.LoadDefaultConfig(ctx, opts...)
	if err != nil {
		return aws.Config{}, fmt.Errorf("loading AWS config: %w", err)
	}

	// Handle role assumption
	if awsCfg.Credentials.Role != "" {
		stsClient := sts.NewFromConfig(cfg)
		provider := stscreds.NewAssumeRoleProvider(stsClient, awsCfg.Credentials.Role, func(o *stscreds.AssumeRoleOptions) {
			if awsCfg.Credentials.RoleExternalID != "" {
				o.ExternalID = &awsCfg.Credentials.RoleExternalID
			}
		})
		cfg.Credentials = provider
	}

	return cfg, nil
}

// ProcessAWSTags converts AWS tags to metadata based on configuration
func ProcessAWSTags(tagsToMetadata bool, includeTags []string, tags map[string]string) map[string]interface{} {
	metadata := make(map[string]interface{})

	if !tagsToMetadata || len(tags) == 0 {
		return metadata
	}

	for key, value := range tags {
		// Skip if tag is not in include list (if specified)
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

// ShouldIncludeResource checks if a resource should be included based on filter patterns
func ShouldIncludeResource(name string, filter Filter) bool {
	// If no filters are specified, include everything
	if len(filter.Include) == 0 && len(filter.Exclude) == 0 {
		return true
	}

	for _, pattern := range filter.Exclude {
		matched, err := regexp.MatchString(pattern, name)
		if err == nil && matched {
			return false
		}
	}

	if len(filter.Include) == 0 {
		return true
	}

	for _, pattern := range filter.Include {
		matched, err := regexp.MatchString(pattern, name)
		if err == nil && matched {
			return true
		}
	}

	return false
}
