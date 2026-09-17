package vertexai

import (
	pluginsdk "github.com/marmotdata/plugin-sdk"
	"github.com/marmotdata/plugin-sdk/mrn"
	"github.com/rs/zerolog/log"
	"google.golang.org/api/aiplatform/v1"
)

// modelAssets catalogues the model registry, links each model to the
// Cloud Storage bucket holding its artifact, and to the pipeline job that
// produced it when that job was discovered in this run.
func (s *Source) modelAssets(items []scanned[*aiplatform.GoogleCloudAiplatformV1Model], jobs index, edges *edgeSet) ([]pluginsdk.Asset, index) {
	displayNames := make([]string, 0, len(items))
	for _, item := range items {
		displayNames = append(displayNames, item.resource.DisplayName)
	}
	naming := newNames(displayNames)

	assets := make([]pluginsdk.Asset, 0, len(items))
	models := index{}

	for _, item := range items {
		model := item.resource
		id := s.locate(model.Name, item.location)

		name := naming.resolve(model.DisplayName, id.id)
		if name == "" {
			log.Warn().Str("resource_name", model.Name).Msg("Skipping a model with no display name or id")
			continue
		}

		asset := s.newAsset("Model", name, model.Description, s.modelMetadata(model, id))
		assets = append(assets, asset)
		models[model.Name] = name

		// The bucket belongs to the GCS plugin, so only the edge is
		// emitted here and the server drops it when no such bucket is
		// catalogued.
		if bucket := gcsBucket(model.ArtifactUri); bucket != "" {
			edges.add(mrn.New("Bucket", "GCS", bucket), *asset.MRN, "FEEDS")
		}

		for _, source := range []string{model.PipelineJob, model.TrainingPipeline} {
			job, ok := jobs[source]
			if !ok {
				continue
			}
			edges.add(assetMRN("Job", job), *asset.MRN, "PRODUCES")
		}
	}

	log.Debug().Int("count", len(assets)).Msg("Discovered models")
	return assets, models
}

func (s *Source) modelMetadata(model *aiplatform.GoogleCloudAiplatformV1Model, id resourceName) map[string]any {
	metadata := commonMetadata(id, model.CreateTime, model.UpdateTime, model.Labels)

	putString(metadata, "display_name", model.DisplayName)
	putString(metadata, "version_id", model.VersionId)
	putStrings(metadata, "version_aliases", model.VersionAliases)
	putString(metadata, "version_description", model.VersionDescription)
	putString(metadata, "version_create_time", model.VersionCreateTime)
	putString(metadata, "artifact_uri", model.ArtifactUri)
	putString(metadata, "metadata_schema_uri", model.MetadataSchemaUri)

	if container := model.ContainerSpec; container != nil {
		putString(metadata, "container_image", container.ImageUri)
	}

	if schemata := model.PredictSchemata; schemata != nil {
		putString(metadata, "predict_schemata_instance", schemata.InstanceSchemaUri)
		putString(metadata, "predict_schemata_parameters", schemata.ParametersSchemaUri)
		putString(metadata, "predict_schemata_prediction", schemata.PredictionSchemaUri)
	}

	putStrings(metadata, "supported_deployment_resources_types", model.SupportedDeploymentResourcesTypes)
	putStrings(metadata, "supported_input_storage_formats", model.SupportedInputStorageFormats)
	putStrings(metadata, "supported_output_storage_formats", model.SupportedOutputStorageFormats)

	// The API gives these as full resource names. The bare id is what the
	// pipeline job asset is named after and what a person reads.
	putString(metadata, "training_pipeline", resourceID(model.TrainingPipeline))
	putString(metadata, "pipeline_job", resourceID(model.PipelineJob))

	putString(metadata, "base_model_source", baseModelSource(model.BaseModelSource))
	metadata["deployed_model_count"] = len(model.DeployedModels)

	return metadata
}

// baseModelSource names the model this one was derived from. A Model
// Garden model is identified by its public name, a Genie model by the URI
// of its base model.
func baseModelSource(source *aiplatform.GoogleCloudAiplatformV1ModelBaseModelSource) string {
	if source == nil {
		return ""
	}
	if source.ModelGardenSource != nil && source.ModelGardenSource.PublicModelName != "" {
		return source.ModelGardenSource.PublicModelName
	}
	if source.GenieSource != nil {
		return source.GenieSource.BaseModelUri
	}
	return ""
}
