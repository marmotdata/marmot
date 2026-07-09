package plugin

import (
	"context"
	"os"
	"path/filepath"
	"regexp"

	"github.com/aws/aws-sdk-go-v2/config"
)

type Filter struct {
	Include []string `json:"include,omitempty" description:"Include patterns for resource names (regex)"`
	Exclude []string `json:"exclude,omitempty" description:"Exclude patterns for resource names (regex)"`
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
} // @name AWSCredentialStatus

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
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
