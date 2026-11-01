package sagemaker

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/service/sagemaker"
	"github.com/aws/aws-sdk-go-v2/service/sagemaker/types"
	pluginsdk "github.com/marmotdata/plugin-sdk"
	"github.com/marmotdata/plugin-sdk/mrn"
	"github.com/rs/zerolog/log"
)

// jobNamespace groups this plugin's run history under one name in the
// Marmot run history view.
const jobNamespace = "sagemaker"

// discoverTrainingJobs catalogues training runs, the data they read and the
// model artifact they produced.
func (s *Source) discoverTrainingJobs(ctx context.Context, index *modelIndex) ([]pluginsdk.Asset, []pluginsdk.LineageEdge, []pluginsdk.AssetRunHistory, error) {
	summaries, err := s.listTrainingJobs(ctx)
	if err != nil {
		return nil, nil, nil, err
	}

	var assets []pluginsdk.Asset
	var edges []pluginsdk.LineageEdge
	var runs []pluginsdk.AssetRunHistory

	for _, summary := range summaries {
		name := deref(summary.TrainingJobName)
		if name == "" {
			continue
		}

		detail, err := s.client.DescribeTrainingJob(ctx, &sagemaker.DescribeTrainingJobInput{TrainingJobName: &name})
		if err != nil {
			log.Warn().Err(err).Str("training_job", name).Msg("Failed to describe training job")
			continue
		}

		asset := s.trainingJobAsset(ctx, name, detail)
		assets = append(assets, asset)
		edges = append(edges, trainingEdges(asset, detail, index)...)

		if history := runHistory(asset, detail); history != nil {
			runs = append(runs, *history)
		}
	}

	log.Debug().Int("count", len(assets)).Msg("Discovered training jobs")
	return assets, edges, runs, nil
}

func (s *Source) trainingJobAsset(ctx context.Context, name string, detail *sagemaker.DescribeTrainingJobOutput) pluginsdk.Asset {
	arn := deref(detail.TrainingJobArn)

	metadata := s.resourceTags(ctx, arn)
	if metadata == nil {
		metadata = map[string]any{}
	}

	if arn != "" {
		metadata["arn"] = arn
	}
	if detail.TrainingJobStatus != "" {
		metadata["status"] = string(detail.TrainingJobStatus)
	}
	putString(metadata, "failure_reason", detail.FailureReason)

	if algo := detail.AlgorithmSpecification; algo != nil {
		putString(metadata, "algorithm_image", algo.TrainingImage)
		putString(metadata, "algorithm_name", algo.AlgorithmName)
	}

	if len(detail.HyperParameters) > 0 {
		hyperParameters := make(map[string]any, len(detail.HyperParameters))
		for key, value := range detail.HyperParameters {
			hyperParameters[key] = value
		}
		metadata["hyperparameters"] = hyperParameters
	}

	if res := detail.ResourceConfig; res != nil {
		if res.InstanceType != "" {
			metadata["instance_type"] = string(res.InstanceType)
		}
		if res.InstanceCount != nil {
			metadata["instance_count"] = *res.InstanceCount
		}
	}

	if channels := inputChannels(detail.InputDataConfig); len(channels) > 0 {
		metadata["input_channels"] = channels
	}
	if out := detail.OutputDataConfig; out != nil {
		putString(metadata, "output_s3_path", out.S3OutputPath)
	}
	if artifacts := detail.ModelArtifacts; artifacts != nil {
		putString(metadata, "model_artifacts_s3_uri", artifacts.S3ModelArtifacts)
	}

	if metrics := finalMetrics(detail.FinalMetricDataList); len(metrics) > 0 {
		metadata["final_metrics"] = metrics
	}

	putTime(metadata, "started_at", detail.TrainingStartTime)
	putTime(metadata, "ended_at", detail.TrainingEndTime)
	putTime(metadata, "created_at", detail.CreationTime)

	if s.region != "" {
		metadata["region"] = s.region
	}
	if url := s.consoleURL("jobs/" + name); url != "" {
		metadata["url"] = url
	}

	return s.newAsset("Job", name, "", metadata)
}

// inputChannels maps each named training channel to the S3 location it
// reads from.
func inputChannels(channels []types.Channel) map[string]any {
	out := map[string]any{}

	for _, c := range channels {
		name := deref(c.ChannelName)
		if name == "" || c.DataSource == nil || c.DataSource.S3DataSource == nil {
			continue
		}
		if uri := deref(c.DataSource.S3DataSource.S3Uri); uri != "" {
			out[name] = uri
		}
	}

	return out
}

// finalMetrics records the last value the job reported for each metric it
// emitted, for example the final validation loss.
func finalMetrics(metrics []types.MetricData) map[string]any {
	out := map[string]any{}

	for _, m := range metrics {
		name := deref(m.MetricName)
		if name == "" || m.Value == nil {
			continue
		}
		out[name] = *m.Value
	}

	return out
}

// trainingEdges links a job to the data it read and the model it produced.
// The artifact match is how SageMaker itself connects the two: a model
// points at the tarball a training job wrote.
func trainingEdges(job pluginsdk.Asset, detail *sagemaker.DescribeTrainingJobOutput, index *modelIndex) []pluginsdk.LineageEdge {
	var edges []pluginsdk.LineageEdge

	if channels, ok := job.Metadata["input_channels"].(map[string]any); ok {
		seen := map[string]bool{}
		for _, uri := range channels {
			bucket := s3Bucket(fmt.Sprint(uri))
			if bucket == "" || seen[bucket] {
				continue
			}
			seen[bucket] = true
			edges = append(edges, pluginsdk.LineageEdge{
				Source: mrn.New("Bucket", "S3", bucket),
				Target: *job.MRN,
				Type:   "FEEDS",
			})
		}
	}

	if detail.ModelArtifacts != nil {
		if model, ok := index.byArtifact[deref(detail.ModelArtifacts.S3ModelArtifacts)]; ok {
			edges = append(edges, pluginsdk.LineageEdge{
				Source: *job.MRN,
				Target: assetMRN("Model", model),
				Type:   "PRODUCES",
			})
		}
	}

	return edges
}

// runHistory turns a training job's status and timestamps into the start and
// finish events Marmot shows on the asset's run history.
func runHistory(job pluginsdk.Asset, detail *sagemaker.DescribeTrainingJobOutput) *pluginsdk.AssetRunHistory {
	start := detail.TrainingStartTime
	if start == nil || start.IsZero() {
		start = detail.CreationTime
	}
	if start == nil || start.IsZero() {
		return nil
	}

	name := *job.Name
	facets := map[string]any{"training_job_name": name}
	if status := string(detail.TrainingJobStatus); status != "" {
		facets["status"] = status
	}

	events := []pluginsdk.RunHistoryEvent{{
		RunID:        name,
		JobNamespace: jobNamespace,
		JobName:      name,
		EventType:    "START",
		EventTime:    *start,
		RunFacets:    facets,
	}}

	finalType := trainingEventType(detail.TrainingJobStatus)
	if finalType != "" {
		end := detail.TrainingEndTime
		if end == nil || end.IsZero() {
			end = start
		}
		events = append(events, pluginsdk.RunHistoryEvent{
			RunID:        name,
			JobNamespace: jobNamespace,
			JobName:      name,
			EventType:    finalType,
			EventTime:    *end,
			RunFacets:    facets,
		})
	}

	return &pluginsdk.AssetRunHistory{AssetMRN: *job.MRN, Runs: events}
}

// trainingEventType maps a training job status onto the event that closes
// its run. A job that has not finished reports "", leaving only the start
// event, which is how Marmot recognises a run as still going.
func trainingEventType(status types.TrainingJobStatus) string {
	switch status {
	case types.TrainingJobStatusCompleted:
		return "COMPLETE"
	case types.TrainingJobStatusFailed:
		return "FAIL"
	case types.TrainingJobStatusStopped:
		return "ABORT"
	default:
		return ""
	}
}
