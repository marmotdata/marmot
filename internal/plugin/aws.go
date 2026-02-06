package plugin

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/credentials/stscreds"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	"sigs.k8s.io/yaml"
)

type AWSCredentials struct {
	UseDefault     bool   `json:"use_default,omitempty" description:"Use AWS credentials from environment or default profile (recommended)"`
	ID             string `json:"id,omitempty" description:"AWS access key ID"`
	Secret         string `json:"secret,omitempty" description:"AWS secret access key" sensitive:"true"`
	Token          string `json:"token,omitempty" description:"AWS session token" sensitive:"true"`
	Profile        string `json:"profile,omitempty" description:"AWS profile to use from shared credentials file"`
	Role           string `json:"role,omitempty" description:"AWS IAM role ARN to assume"`
	RoleExternalID string `json:"role_external_id,omitempty" description:"External ID for cross-account role assumption"`
	Region         string `json:"region,omitempty" description:"AWS region for services"`
	Endpoint       string `json:"endpoint,omitempty" description:"Custom endpoint URL for AWS services" validate:"omitempty,url"`
}

type Filter struct {
	Include []string `json:"include,omitempty" description:"Include patterns for resource names (regex)"`
	Exclude []string `json:"exclude,omitempty" description:"Exclude patterns for resource names (regex)"`
}

type AWSConfig struct {
	Credentials    AWSCredentials `json:"credentials" description:"AWS credentials configuration"`
	TagsToMetadata bool           `json:"tags_to_metadata,omitempty" description:"Convert AWS tags to Marmot metadata"`
	IncludeTags    []string       `json:"include_tags,omitempty" description:"List of AWS tags to include as metadata. By default, all tags are included."`
}

func (a *AWSConfig) Validate() error {
	return nil
}

type AWSPlugin struct {
	AWSConfig  `json:",inline"`
	BaseConfig `json:",inline"`
}

// ErrEndpointNotFound is returned when an endpoint can't be resolved
var ErrEndpointNotFound = fmt.Errorf("endpoint not found")

func ExtractAWSConfig(rawConfig map[string]interface{}) (*AWSConfig, error) {
	var awsCfg AWSConfig
	configBytes, err := yaml.Marshal(rawConfig)
	if err != nil {
		return nil, fmt.Errorf("marshaling raw config: %w", err)
	}

	if err := yaml.Unmarshal(configBytes, &awsCfg); err != nil {
		return nil, fmt.Errorf("unmarshaling into AWSConfig: %w", err)
	}

	return &awsCfg, nil
}

// DetectAWSCredentials checks if AWS credentials are available from environment or config files
func DetectAWSCredentials(ctx context.Context) *AWSCredentialStatus {
	status := &AWSCredentialStatus{
		Available: false,
		Sources:   []string{},
	}

	// Check environment variables
	if os.Getenv("AWS_ACCESS_KEY_ID") != "" && os.Getenv("AWS_SECRET_ACCESS_KEY") != "" {
		status.Available = true
		status.Sources = append(status.Sources, "environment variables")
	}

	// Check for credentials file
	homeDir, err := os.UserHomeDir()
	if err == nil {
		credsPath := filepath.Join(homeDir, ".aws", "credentials")
		if _, err := os.Stat(credsPath); err == nil {
			status.Available = true
			status.Sources = append(status.Sources, "credentials file (~/.aws/credentials)")
		}
	}

	// Check for config file with profile
	if err == nil {
		configPath := filepath.Join(homeDir, ".aws", "config")
		if _, err := os.Stat(configPath); err == nil {
			if !contains(status.Sources, "credentials file (~/.aws/credentials)") {
				status.Available = true
				status.Sources = append(status.Sources, "config file (~/.aws/config)")
			}
		}
	}

	// Check for EC2 instance metadata (IMDS)
	if os.Getenv("AWS_CONTAINER_CREDENTIALS_RELATIVE_URI") != "" || os.Getenv("AWS_CONTAINER_CREDENTIALS_FULL_URI") != "" {
		status.Available = true
		status.Sources = append(status.Sources, "container credentials")
	}

	// Try to actually load the config to verify credentials work
	if status.Available {
		_, err := config.LoadDefaultConfig(ctx)
		if err != nil {
			status.Available = false
			status.Error = err.Error()
		}
	}

	return status
}

type AWSCredentialStatus struct {
	Available bool     `json:"available"`
	Sources   []string `json:"sources"`
	Error     string   `json:"error,omitempty"`
}

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

func (a *AWSConfig) NewAWSConfig(ctx context.Context) (aws.Config, error) {
	var opts []func(*config.LoadOptions) error

	if a.Credentials.Region != "" {
		opts = append(opts, config.WithRegion(a.Credentials.Region))
	}

	// If UseDefault is true or no explicit credentials provided, use default credential chain
	if a.Credentials.UseDefault || (a.Credentials.ID == "" && a.Credentials.Profile == "") {
		// Just load default config - will follow AWS credential chain
		if a.Credentials.Profile != "" {
			opts = append(opts, config.WithSharedConfigProfile(a.Credentials.Profile))
		}
	} else {
		// Use explicit credentials if provided
		if a.Credentials.ID != "" && a.Credentials.Secret != "" {
			provider := credentials.NewStaticCredentialsProvider(
				a.Credentials.ID,
				a.Credentials.Secret,
				a.Credentials.Token,
			)
			opts = append(opts, config.WithCredentialsProvider(provider))
		}

		if a.Credentials.Profile != "" {
			opts = append(opts, config.WithSharedConfigProfile(a.Credentials.Profile))
		}
	}

	cfg, err := config.LoadDefaultConfig(ctx, opts...)
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

func ProcessAWSTags(tagsToMetadata bool, includeTags []string, tags map[string]string) map[string]interface{} {
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

func ShouldIncludeResource(name string, filter Filter) bool {
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
