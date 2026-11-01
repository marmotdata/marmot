package sagemaker

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/service/sagemaker"
	"github.com/aws/aws-sdk-go-v2/service/sagemaker/types"
	pluginsdk "github.com/marmotdata/plugin-sdk"
	"github.com/rs/zerolog/log"
)

// discoverEndpoints catalogues the inference endpoints models are served
// from, and links each endpoint back to the models behind its variants.
func (s *Source) discoverEndpoints(ctx context.Context, index *modelIndex) ([]pluginsdk.Asset, []pluginsdk.LineageEdge, error) {
	summaries, err := s.listEndpoints(ctx)
	if err != nil {
		return nil, nil, err
	}

	var assets []pluginsdk.Asset
	var edges []pluginsdk.LineageEdge

	for _, summary := range summaries {
		name := deref(summary.EndpointName)
		if name == "" {
			continue
		}

		asset, err := s.endpointAsset(ctx, name, summary)
		if err != nil {
			log.Warn().Err(err).Str("endpoint", name).Msg("Failed to describe endpoint")
			continue
		}
		assets = append(assets, asset)
		edges = append(edges, servingEdges(asset, index)...)
	}

	log.Debug().Int("count", len(assets)).Msg("Discovered endpoints")
	return assets, edges, nil
}

func (s *Source) endpointAsset(ctx context.Context, name string, summary types.EndpointSummary) (pluginsdk.Asset, error) {
	detail, err := s.client.DescribeEndpoint(ctx, &sagemaker.DescribeEndpointInput{EndpointName: &name})
	if err != nil {
		return pluginsdk.Asset{}, fmt.Errorf("describing endpoint: %w", err)
	}

	arn := deref(summary.EndpointArn)
	if arn == "" {
		arn = deref(detail.EndpointArn)
	}

	metadata := s.resourceTags(ctx, arn)
	if metadata == nil {
		metadata = map[string]any{}
	}

	if arn != "" {
		metadata["arn"] = arn
	}
	if detail.EndpointStatus != "" {
		metadata["status"] = string(detail.EndpointStatus)
	}
	putString(metadata, "endpoint_config", detail.EndpointConfigName)
	putTime(metadata, "created_at", detail.CreationTime)
	putTime(metadata, "last_modified_at", detail.LastModifiedTime)

	if config := deref(detail.EndpointConfigName); config != "" {
		s.addEndpointConfigMetadata(ctx, name, config, metadata)
	}

	if s.region != "" {
		metadata["region"] = s.region
	}
	if url := s.consoleURL("endpoints/" + name); url != "" {
		metadata["url"] = url
	}

	return s.newAsset("Endpoint", name, "", metadata), nil
}

// addEndpointConfigMetadata records what the endpoint actually serves. The
// endpoint configuration, not the endpoint, holds the model behind each
// variant and the hardware it runs on.
func (s *Source) addEndpointConfigMetadata(ctx context.Context, endpoint, config string, metadata map[string]any) {
	detail, err := s.client.DescribeEndpointConfig(ctx, &sagemaker.DescribeEndpointConfigInput{
		EndpointConfigName: &config,
	})
	if err != nil {
		log.Warn().Err(err).Str("endpoint", endpoint).Str("endpoint_config", config).
			Msg("Failed to describe endpoint configuration")
		return
	}

	if variants := variantList(detail.ProductionVariants); len(variants) > 0 {
		metadata["variants"] = variants
	}

	if capture := detail.DataCaptureConfig; capture != nil {
		putString(metadata, "data_capture_s3_uri", capture.DestinationS3Uri)
	}
	putString(metadata, "kms_key_id", detail.KmsKeyId)
}

// variantList flattens the production variants an endpoint serves traffic
// from, one entry per variant.
func variantList(variants []types.ProductionVariant) []map[string]any {
	var out []map[string]any

	for _, v := range variants {
		entry := map[string]any{}
		putString(entry, "name", v.VariantName)
		putString(entry, "model", v.ModelName)
		if v.InstanceType != "" {
			entry["instance_type"] = string(v.InstanceType)
		}
		if v.InitialInstanceCount != nil {
			entry["instance_count"] = *v.InitialInstanceCount
		}
		if v.InitialVariantWeight != nil {
			entry["weight"] = *v.InitialVariantWeight
		}
		// A serverless variant has no instance type or count, so record the
		// flag that explains why those fields are missing.
		if v.ServerlessConfig != nil {
			entry["serverless"] = true
		}
		if len(entry) > 0 {
			out = append(out, entry)
		}
	}

	return out
}

// servingEdges links every model an endpoint serves to that endpoint. A
// variant can name a model in another account, so only models this run
// catalogued get an edge.
func servingEdges(endpoint pluginsdk.Asset, index *modelIndex) []pluginsdk.LineageEdge {
	variants, ok := endpoint.Metadata["variants"].([]map[string]any)
	if !ok {
		return nil
	}

	var edges []pluginsdk.LineageEdge
	seen := map[string]bool{}

	for _, variant := range variants {
		model, _ := variant["model"].(string)
		if model == "" || seen[model] || !index.names[model] {
			continue
		}
		seen[model] = true
		edges = append(edges, pluginsdk.LineageEdge{
			Source: assetMRN("Model", model),
			Target: *endpoint.MRN,
			Type:   "FEEDS",
		})
	}

	return edges
}
