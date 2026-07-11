// Package lambda discovers Lambda functions from AWS accounts.
package lambda

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/lambda"
	"github.com/aws/aws-sdk-go-v2/service/lambda/types"
	pluginsdk "github.com/marmotdata/plugin-sdk"
	"github.com/marmotdata/plugin-sdk/mrn"
	"github.com/rs/zerolog/log"
)

// Meta describes the plugin to the Marmot host.
func Meta() pluginsdk.Meta {
	return pluginsdk.Meta{
		ID:          "lambda",
		Name:        "AWS Lambda",
		Description: "Discover Lambda functions from AWS accounts",
		Icon:        "lambda",
		Category:    "compute",
		Status:      "experimental",
		Features:    []string{"Assets"},
		ConfigSpec:  pluginsdk.GenerateConfigSpec(Config{}),
	}
}

// Config for Lambda plugin
type Config struct {
	pluginsdk.BaseConfig `json:",inline"`
	*pluginsdk.AWSConfig `json:",inline"`
}

// Example configuration for the plugin
var _ = `
credentials:
  region: "us-east-1"
  profile: "production"
  role: "<role>"
tags:
  - "aws"
`

type Source struct {
	config *Config
	client *lambda.Client
}

func (s *Source) Validate(rawConfig pluginsdk.RawConfig) (pluginsdk.RawConfig, error) {
	config, err := pluginsdk.UnmarshalConfig[Config](rawConfig)
	if err != nil {
		return nil, fmt.Errorf("unmarshaling config: %w", err)
	}

	if err := pluginsdk.ValidateStruct(config); err != nil {
		return nil, err
	}

	s.config = config
	return rawConfig, nil
}

func (s *Source) Discover(ctx context.Context, pluginConfig pluginsdk.RawConfig) (*pluginsdk.DiscoveryResult, error) {
	config, err := pluginsdk.UnmarshalConfig[Config](pluginConfig)
	if err != nil {
		return nil, fmt.Errorf("unmarshaling config: %w", err)
	}
	s.config = config

	awsConfig, err := pluginsdk.ExtractAWSConfig(pluginConfig)
	if err != nil {
		return nil, fmt.Errorf("extracting AWS config: %w", err)
	}

	awsCfg, err := awsConfig.NewAWSConfig(ctx)
	if err != nil {
		return nil, fmt.Errorf("creating AWS config: %w", err)
	}

	s.client = lambda.NewFromConfig(awsCfg)

	functions, err := s.discoverFunctions(ctx)
	if err != nil {
		return nil, fmt.Errorf("discovering functions: %w", err)
	}

	var assets []pluginsdk.Asset
	for _, fn := range functions {
		a, err := s.createFunctionAsset(ctx, fn)
		if err != nil {
			log.Warn().Err(err).Str("function", *fn.FunctionName).Msg("Failed to create asset for function")
			continue
		}
		assets = append(assets, a)
	}

	return &pluginsdk.DiscoveryResult{
		Assets: assets,
	}, nil
}

func (s *Source) discoverFunctions(ctx context.Context) ([]types.FunctionConfiguration, error) {
	var functions []types.FunctionConfiguration
	paginator := lambda.NewListFunctionsPaginator(s.client, &lambda.ListFunctionsInput{})

	for paginator.HasMorePages() {
		output, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("listing functions: %w", err)
		}
		functions = append(functions, output.Functions...)
	}

	return functions, nil
}

func (s *Source) createFunctionAsset(ctx context.Context, fn types.FunctionConfiguration) (pluginsdk.Asset, error) {
	metadata := make(map[string]interface{})
	functionName := *fn.FunctionName

	// Collect tags first
	if s.config.AWSConfig != nil && s.config.TagsToMetadata {
		tagsOutput, err := s.client.ListTags(ctx, &lambda.ListTagsInput{
			Resource: fn.FunctionArn,
		})
		if err != nil {
			log.Warn().Err(err).Str("function", functionName).Msg("Failed to get function tags")
		} else {
			metadata = pluginsdk.ProcessAWSTags(s.config.TagsToMetadata, s.config.IncludeTags, tagsOutput.Tags)
		}
	}

	// Function identity
	if fn.FunctionArn != nil {
		metadata["function_arn"] = *fn.FunctionArn
	}
	metadata["runtime"] = string(fn.Runtime)
	if fn.Handler != nil {
		metadata["handler"] = *fn.Handler
	}
	if fn.Role != nil {
		metadata["role"] = *fn.Role
	}

	// Code
	metadata["code_size"] = fn.CodeSize
	if fn.CodeSha256 != nil {
		metadata["code_sha256"] = *fn.CodeSha256
	}
	metadata["package_type"] = string(fn.PackageType)

	// Configuration
	if fn.MemorySize != nil {
		metadata["memory_size_mb"] = *fn.MemorySize
	}
	if fn.Timeout != nil {
		metadata["timeout_seconds"] = *fn.Timeout
	}
	if fn.Description != nil && *fn.Description != "" {
		metadata["description"] = *fn.Description
	}
	if fn.LastModified != nil {
		metadata["last_modified"] = *fn.LastModified
	}
	if fn.Version != nil {
		metadata["version"] = *fn.Version
	}

	// Architecture
	if len(fn.Architectures) > 0 {
		var archs []string
		for _, arch := range fn.Architectures {
			archs = append(archs, string(arch))
		}
		metadata["architectures"] = strings.Join(archs, ", ")
	}

	// Environment variable count (not values, for security)
	if fn.Environment != nil && fn.Environment.Variables != nil {
		metadata["environment_variable_count"] = len(fn.Environment.Variables)
	}

	// VPC config
	if fn.VpcConfig != nil && fn.VpcConfig.VpcId != nil && *fn.VpcConfig.VpcId != "" {
		metadata["vpc_id"] = *fn.VpcConfig.VpcId
		metadata["subnet_count"] = len(fn.VpcConfig.SubnetIds)
		metadata["security_group_count"] = len(fn.VpcConfig.SecurityGroupIds)
	}

	// Ephemeral storage
	if fn.EphemeralStorage != nil && fn.EphemeralStorage.Size != nil {
		metadata["ephemeral_storage_mb"] = *fn.EphemeralStorage.Size
	}

	// Layers
	if len(fn.Layers) > 0 {
		var layerNames []string
		for _, layer := range fn.Layers {
			if layer.Arn != nil {
				layerNames = append(layerNames, *layer.Arn)
			}
		}
		metadata["layers"] = strings.Join(layerNames, ", ")
		metadata["layer_count"] = len(fn.Layers)
	}

	// Tracing
	if fn.TracingConfig != nil {
		metadata["tracing_mode"] = string(fn.TracingConfig.Mode)
	}

	// State
	metadata["state"] = string(fn.State)
	if fn.LastUpdateStatus != "" {
		metadata["last_update_status"] = string(fn.LastUpdateStatus)
	}

	mrnValue := mrn.New("Function", "Lambda", functionName)

	processedTags := pluginsdk.InterpolateTags(s.config.Tags, metadata)

	return pluginsdk.Asset{
		Name:      &functionName,
		MRN:       &mrnValue,
		Type:      "Function",
		Providers: []string{"Lambda"},
		Metadata:  metadata,
		Tags:      processedTags,
		Sources: []pluginsdk.AssetSource{{
			Name:       "Lambda",
			LastSyncAt: time.Now(),
			Properties: metadata,
			Priority:   1,
		}},
	}, nil
}
