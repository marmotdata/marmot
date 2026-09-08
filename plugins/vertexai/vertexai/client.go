package vertexai

import (
	"context"
	"errors"
	"fmt"

	"github.com/rs/zerolog/log"
	"google.golang.org/api/aiplatform/v1"
	"google.golang.org/api/option"
)

// defaultEndpoint is the API's own host, which serves the global location.
const defaultEndpoint = "https://aiplatform.googleapis.com/"

// maxPages stops discovery from walking an unbounded result set. At the
// page size below this still covers ten thousand resources of one kind.
const maxPages = 100

// pageSize is how many resources one list call asks for.
const pageSize = 100

// errPageLimit stops a paginated call once the page cap is reached, and
// errLimit once the caller has as many resources as it asked for. Neither
// leaves this file.
var (
	errPageLimit = errors.New("page limit reached")
	errLimit     = errors.New("requested limit reached")
)

// regionalEndpoint is the host a location's Vertex AI resources are served
// from. The API's default host does not serve them, so a call sent there
// finds nothing. The shapes come from the aiplatform discovery document,
// which lists an endpoint per location.
func regionalEndpoint(location string) string {
	switch location {
	case "", "global":
		// The global location has no endpoint entry of its own; it is
		// served from the API's default host.
		return defaultEndpoint
	case "us", "eu":
		// The two multi-region endpoints spell the host the other way
		// round, as aiplatform.<location>.rep.googleapis.com.
		return fmt.Sprintf("https://aiplatform.%s.rep.googleapis.com/", location)
	default:
		return fmt.Sprintf("https://%s-aiplatform.googleapis.com/", location)
	}
}

// serviceFor builds a client for one location. Vertex AI serves regional
// resources from the location's own host, so each location needs its own
// client, unless the config pins an endpoint for every location at once.
func (s *Source) serviceFor(ctx context.Context, location string) (*aiplatform.Service, error) {
	endpoint := s.config.Endpoint
	if endpoint == "" {
		endpoint = regionalEndpoint(location)
	}

	opts := []option.ClientOption{option.WithEndpoint(endpoint)}

	switch {
	case s.config.DisableAuth:
		opts = append(opts, option.WithoutAuthentication())
	case s.config.CredentialsJSON != "":
		opts = append(opts, option.WithCredentialsJSON([]byte(s.config.CredentialsJSON)))
	case s.config.CredentialsFile != "":
		opts = append(opts, option.WithCredentialsFile(s.config.CredentialsFile))
	}

	service, err := aiplatform.NewService(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("creating Vertex AI client for %s: %w", location, err)
	}

	return service, nil
}

// pageLimitReached logs the one warning that tells an operator some
// resources were left out.
func pageLimitReached(kind string) {
	log.Warn().Str("resource", kind).Int("max_pages", maxPages).
		Msg("Stopped paginating at the page limit, some resources were not discovered")
}

func listModels(ctx context.Context, service *aiplatform.Service, parent string) ([]*aiplatform.GoogleCloudAiplatformV1Model, error) {
	var models []*aiplatform.GoogleCloudAiplatformV1Model
	pages := 0

	err := service.Projects.Locations.Models.List(parent).PageSize(pageSize).
		Pages(ctx, func(page *aiplatform.GoogleCloudAiplatformV1ListModelsResponse) error {
			models = append(models, page.Models...)
			pages++
			if pages == maxPages {
				return errPageLimit
			}
			return nil
		})
	if errors.Is(err, errPageLimit) {
		pageLimitReached("models")
		return models, nil
	}
	if err != nil {
		return nil, fmt.Errorf("listing models: %w", err)
	}

	return models, nil
}

func listEndpoints(ctx context.Context, service *aiplatform.Service, parent string) ([]*aiplatform.GoogleCloudAiplatformV1Endpoint, error) {
	var endpoints []*aiplatform.GoogleCloudAiplatformV1Endpoint
	pages := 0

	err := service.Projects.Locations.Endpoints.List(parent).PageSize(pageSize).
		Pages(ctx, func(page *aiplatform.GoogleCloudAiplatformV1ListEndpointsResponse) error {
			endpoints = append(endpoints, page.Endpoints...)
			pages++
			if pages == maxPages {
				return errPageLimit
			}
			return nil
		})
	if errors.Is(err, errPageLimit) {
		pageLimitReached("endpoints")
		return endpoints, nil
	}
	if err != nil {
		return nil, fmt.Errorf("listing endpoints: %w", err)
	}

	return endpoints, nil
}

func listDatasets(ctx context.Context, service *aiplatform.Service, parent string) ([]*aiplatform.GoogleCloudAiplatformV1Dataset, error) {
	var datasets []*aiplatform.GoogleCloudAiplatformV1Dataset
	pages := 0

	err := service.Projects.Locations.Datasets.List(parent).PageSize(pageSize).
		Pages(ctx, func(page *aiplatform.GoogleCloudAiplatformV1ListDatasetsResponse) error {
			datasets = append(datasets, page.Datasets...)
			pages++
			if pages == maxPages {
				return errPageLimit
			}
			return nil
		})
	if errors.Is(err, errPageLimit) {
		pageLimitReached("datasets")
		return datasets, nil
	}
	if err != nil {
		return nil, fmt.Errorf("listing datasets: %w", err)
	}

	return datasets, nil
}

func listFeatureGroups(ctx context.Context, service *aiplatform.Service, parent string) ([]*aiplatform.GoogleCloudAiplatformV1FeatureGroup, error) {
	var groups []*aiplatform.GoogleCloudAiplatformV1FeatureGroup
	pages := 0

	err := service.Projects.Locations.FeatureGroups.List(parent).PageSize(pageSize).
		Pages(ctx, func(page *aiplatform.GoogleCloudAiplatformV1ListFeatureGroupsResponse) error {
			groups = append(groups, page.FeatureGroups...)
			pages++
			if pages == maxPages {
				return errPageLimit
			}
			return nil
		})
	if errors.Is(err, errPageLimit) {
		pageLimitReached("feature groups")
		return groups, nil
	}
	if err != nil {
		return nil, fmt.Errorf("listing feature groups: %w", err)
	}

	return groups, nil
}

// listFeatures reads the features of one group. The parent is the group's
// own resource name.
func listFeatures(ctx context.Context, service *aiplatform.Service, group string) ([]*aiplatform.GoogleCloudAiplatformV1Feature, error) {
	var features []*aiplatform.GoogleCloudAiplatformV1Feature
	pages := 0

	err := service.Projects.Locations.FeatureGroups.Features.List(group).PageSize(pageSize).
		Pages(ctx, func(page *aiplatform.GoogleCloudAiplatformV1ListFeaturesResponse) error {
			features = append(features, page.Features...)
			pages++
			if pages == maxPages {
				return errPageLimit
			}
			return nil
		})
	if errors.Is(err, errPageLimit) {
		pageLimitReached("features")
		return features, nil
	}
	if err != nil {
		return nil, fmt.Errorf("listing features: %w", err)
	}

	return features, nil
}

// listPipelineJobs reads the most recent jobs first and stops at limit. A
// project keeps every job it has ever run, so reading them all would be
// slow and mostly historical.
func listPipelineJobs(ctx context.Context, service *aiplatform.Service, parent string, limit int) ([]*aiplatform.GoogleCloudAiplatformV1PipelineJob, error) {
	var jobs []*aiplatform.GoogleCloudAiplatformV1PipelineJob
	pages := 0

	size := int64(pageSize)
	if limit > 0 && int64(limit) < size {
		size = int64(limit)
	}

	err := service.Projects.Locations.PipelineJobs.List(parent).
		OrderBy("create_time desc").PageSize(size).
		Pages(ctx, func(page *aiplatform.GoogleCloudAiplatformV1ListPipelineJobsResponse) error {
			jobs = append(jobs, page.PipelineJobs...)
			if limit > 0 && len(jobs) >= limit {
				jobs = jobs[:limit]
				return errLimit
			}
			pages++
			if pages == maxPages {
				return errPageLimit
			}
			return nil
		})
	if errors.Is(err, errLimit) {
		return jobs, nil
	}
	if errors.Is(err, errPageLimit) {
		pageLimitReached("pipeline jobs")
		return jobs, nil
	}
	if err != nil {
		return nil, fmt.Errorf("listing pipeline jobs: %w", err)
	}

	return jobs, nil
}
