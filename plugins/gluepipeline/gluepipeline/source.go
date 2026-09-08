// Package gluepipeline discovers workflows, their tasks and run history
// from AWS Glue.
package gluepipeline

import (
	"context"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/glue"
	"github.com/aws/aws-sdk-go-v2/service/glue/types"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	pluginsdk "github.com/marmotdata/plugin-sdk"
	"github.com/rs/zerolog/log"
)

// Meta describes the plugin to the Marmot host.
func Meta() pluginsdk.Meta {
	return pluginsdk.Meta{
		ID:          "gluepipeline",
		Name:        "Glue Pipelines",
		Description: "Discover workflows, tasks and run history from AWS Glue",
		Icon:        "glue",
		Category:    "orchestration",
		Status:      "experimental",
		Features:    []string{"Assets", "Lineage", "Run History"},
		ConfigSpec:  pluginsdk.GenerateConfigSpec(Config{}),
	}
}

// Config for Glue Pipelines plugin
type Config struct {
	pluginsdk.BaseConfig `json:",inline"`
	*pluginsdk.AWSConfig `json:",inline"`

	IncludeWorkflows  bool `json:"include_workflows" description:"Whether to discover Glue workflows" default:"true"`
	IncludeTriggers   bool `json:"include_triggers" description:"Whether to read trigger definitions" default:"true"`
	IncludeRunHistory bool `json:"include_run_history" description:"Whether to collect workflow, job and crawler runs" default:"true"`
	RunHistoryLimit   int  `json:"run_history_limit" description:"How many recent runs to read per workflow, job and crawler" default:"20" validate:"min=1,max=200"`
	IncludeCrawlers   bool `json:"include_crawlers" description:"Whether to read crawlers for lineage and runs" default:"true"`
}

// Example configuration for the plugin
var _ = `
credentials:
  region: "us-east-1"
  profile: "production"
  role: "<role>"
tags:
  - "aws"
include_workflows: true
include_triggers: true
include_run_history: true
run_history_limit: 20
include_crawlers: true
tags_to_metadata: true
`

type Source struct {
	config *Config
	client glueAPI

	// region and account build console URLs and the ARNs the tag lookup
	// needs. account stays empty when the caller identity is unavailable.
	region  string
	account string
}

func (s *Source) Validate(rawConfig pluginsdk.RawConfig) (pluginsdk.RawConfig, error) {
	config, err := pluginsdk.UnmarshalConfig[Config](rawConfig)
	if err != nil {
		return nil, fmt.Errorf("unmarshalling config: %w", err)
	}

	pluginsdk.ApplyDefaults(config, rawConfig)

	// An ingest file that takes its AWS credentials from the environment
	// leaves the whole block out, and the rest of the plugin still reads
	// settings from it.
	if config.AWSConfig == nil {
		config.AWSConfig = &pluginsdk.AWSConfig{}
	}

	if err := pluginsdk.ValidateStruct(config); err != nil {
		return nil, err
	}

	s.config = config
	return rawConfig, nil
}

func (s *Source) Discover(ctx context.Context, rawConfig pluginsdk.RawConfig) (*pluginsdk.DiscoveryResult, error) {
	// The host starts a fresh process per call, so Discover configures
	// itself rather than relying on an earlier Validate.
	if _, err := s.Validate(rawConfig); err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(ctx, 10*time.Minute)
	defer cancel()

	awsConfig, err := pluginsdk.ExtractAWSConfig(rawConfig)
	if err != nil {
		return nil, fmt.Errorf("extracting AWS config: %w", err)
	}

	awsCfg, err := awsConfig.NewAWSConfig(ctx)
	if err != nil {
		return nil, fmt.Errorf("creating AWS config: %w", err)
	}

	s.client = glue.NewFromConfig(awsCfg)
	s.region = awsCfg.Region
	s.account = callerAccount(ctx, sts.NewFromConfig(awsCfg))

	return s.discover(ctx)
}

// discover runs against the glueAPI interface so tests can drive it with a
// fake client.
func (s *Source) discover(ctx context.Context) (*pluginsdk.DiscoveryResult, error) {
	result := &pluginsdk.DiscoveryResult{}

	var jobs map[string]types.Job
	if s.config.IncludeWorkflows || s.config.IncludeRunHistory {
		var err error
		jobs, err = s.fetchJobs(ctx)
		if err != nil {
			return nil, fmt.Errorf("reading jobs: %w", err)
		}
	}

	triggers := map[string][]types.Trigger{}
	if s.config.IncludeTriggers {
		var err error
		triggers, err = s.fetchTriggersByWorkflow(ctx)
		if err != nil {
			return nil, fmt.Errorf("reading triggers: %w", err)
		}
	}

	if s.config.IncludeWorkflows {
		workflows, err := s.fetchWorkflows(ctx)
		if err != nil {
			return nil, fmt.Errorf("reading workflows: %w", err)
		}

		for _, workflow := range workflows {
			name := safeStr(workflow.Name)
			if name == "" {
				continue
			}

			nodes := workflowNodes(workflow, triggers[name])
			result.Assets = append(result.Assets, s.pipelineAsset(ctx, workflow, nodes))

			for _, node := range nodes {
				result.Assets = append(result.Assets, s.taskAsset(name, node, jobs))
			}
			result.Lineage = append(result.Lineage, workflowLineage(name, nodes, workflow.Graph)...)

			if s.config.IncludeRunHistory {
				runs, err := s.fetchWorkflowRuns(ctx, name)
				if err != nil {
					log.Warn().Err(err).Str("workflow", name).Msg("Failed to read workflow runs")
					continue
				}
				if history := workflowRunHistory(assetMRN("Pipeline", name), name, runs); len(history.Runs) > 0 {
					result.RunHistory = append(result.RunHistory, history)
				}
			}
		}
	}

	if s.config.IncludeRunHistory {
		for jobName := range jobs {
			runs, err := s.fetchJobRuns(ctx, jobName)
			if err != nil {
				log.Warn().Err(err).Str("job", jobName).Msg("Failed to read job runs")
				continue
			}
			if history := jobRunHistory(assetMRN("Job", jobName), jobName, runs); len(history.Runs) > 0 {
				result.RunHistory = append(result.RunHistory, history)
			}
		}
	}

	if s.config.IncludeCrawlers {
		crawlers, err := s.fetchCrawlers(ctx)
		if err != nil {
			return nil, fmt.Errorf("reading crawlers: %w", err)
		}

		for _, crawler := range crawlers {
			crawlerName := safeStr(crawler.Name)
			if crawlerName == "" {
				continue
			}
			result.Lineage = append(result.Lineage, crawlerLineage(crawler)...)

			if !s.config.IncludeRunHistory {
				continue
			}
			crawls, err := s.fetchCrawls(ctx, crawlerName)
			if err != nil {
				log.Warn().Err(err).Str("crawler", crawlerName).Msg("Failed to read crawler runs")
				continue
			}
			if history := crawlerRunHistory(assetMRN("Crawler", crawlerName), crawlerName, crawls); len(history.Runs) > 0 {
				result.RunHistory = append(result.RunHistory, history)
			}
		}
	}

	return result, nil
}
