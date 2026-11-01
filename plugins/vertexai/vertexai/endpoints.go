package vertexai

import (
	"fmt"
	"sort"
	"strings"

	pluginsdk "github.com/marmotdata/plugin-sdk"
	"github.com/rs/zerolog/log"
	"google.golang.org/api/aiplatform/v1"
)

// endpointAssets catalogues the prediction endpoints models are served
// from, and links each endpoint back to the models deployed behind it.
func (s *Source) endpointAssets(items []scanned[*aiplatform.GoogleCloudAiplatformV1Endpoint], models index, edges *edgeSet) []pluginsdk.Asset {
	displayNames := make([]string, 0, len(items))
	for _, item := range items {
		displayNames = append(displayNames, item.resource.DisplayName)
	}
	naming := newNames(displayNames)

	assets := make([]pluginsdk.Asset, 0, len(items))

	for _, item := range items {
		endpoint := item.resource
		id := s.locate(endpoint.Name, item.location)

		name := naming.resolve(endpoint.DisplayName, id.id)
		if name == "" {
			log.Warn().Str("resource_name", endpoint.Name).Msg("Skipping an endpoint with no display name or id")
			continue
		}

		asset := s.newAsset("Endpoint", name, endpoint.Description, s.endpointMetadata(endpoint, id))
		assets = append(assets, asset)

		for _, deployed := range endpoint.DeployedModels {
			model, ok := models[modelResourceName(deployed.Model)]
			if !ok {
				// A model can be deployed from another project or another
				// location, and its display name is the asset name, so
				// there is nothing to point the edge at.
				log.Debug().Str("endpoint", name).Str("model", deployed.Model).
					Msg("Deployed model was not discovered in this run")
				continue
			}
			edges.add(assetMRN("Model", model), *asset.MRN, "FEEDS")
		}
	}

	log.Debug().Int("count", len(assets)).Msg("Discovered endpoints")
	return assets
}

func (s *Source) endpointMetadata(endpoint *aiplatform.GoogleCloudAiplatformV1Endpoint, id resourceName) map[string]any {
	metadata := commonMetadata(id, endpoint.CreateTime, endpoint.UpdateTime, endpoint.Labels)

	putString(metadata, "display_name", endpoint.DisplayName)
	putString(metadata, "network", endpoint.Network)
	putBool(metadata, "dedicated_endpoint_enabled", endpoint.DedicatedEndpointEnabled)
	putString(metadata, "dedicated_endpoint_dns", endpoint.DedicatedEndpointDns)
	putString(metadata, "traffic_split", trafficSplit(endpoint.TrafficSplit))

	metadata["deployed_model_count"] = len(endpoint.DeployedModels)
	putString(metadata, "deployed_models", deployedModelNames(endpoint.DeployedModels))
	putString(metadata, "model_deployment_monitoring_job", resourceID(endpoint.ModelDeploymentMonitoringJob))

	return metadata
}

// trafficSplit renders how traffic is shared between the models an
// endpoint serves, as "<deployed model id>=<percent>" pairs. Sorting by
// deployed model id keeps the value from changing between runs when the
// split has not.
func trafficSplit(split map[string]int64) string {
	if len(split) == 0 {
		return ""
	}

	ids := make([]string, 0, len(split))
	for id := range split {
		ids = append(ids, id)
	}
	sort.Strings(ids)

	parts := make([]string, 0, len(ids))
	for _, id := range ids {
		parts = append(parts, fmt.Sprintf("%s=%d", id, split[id]))
	}

	return strings.Join(parts, ",")
}

// deployedModelNames lists what an endpoint currently serves, so the
// catalog shows it without following the lineage.
func deployedModelNames(deployed []*aiplatform.GoogleCloudAiplatformV1DeployedModel) string {
	names := make([]string, 0, len(deployed))
	for _, d := range deployed {
		if d.DisplayName == "" {
			continue
		}
		names = append(names, d.DisplayName)
	}
	sort.Strings(names)

	return strings.Join(names, ", ")
}

// modelResourceName drops the version suffix a deployed model may carry,
// because the models list keys on the model, not one of its versions.
func modelResourceName(deployed string) string {
	name, _, _ := strings.Cut(deployed, "@")
	return name
}
