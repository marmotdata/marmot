package firebase

import (
	"context"
	"errors"
	"fmt"
	"time"

	pluginsdk "github.com/marmotdata/plugin-sdk"
	"github.com/rs/zerolog/log"
	firebasedatabase "google.golang.org/api/firebasedatabase/v1beta"
)

// maxInstancePages bounds the Realtime Database listing so a paging bug in
// either end cannot loop forever.
const maxInstancePages = 50

// errPageCap stops the paging helper once the cap is reached. It never
// reaches the caller.
var errPageCap = errors.New("page cap reached")

// discoverRealtimeDatabases catalogues Realtime Database instances as
// containers. Their contents are one untyped JSON tree with no schema to
// infer and no listing API, so only the instances themselves are catalogued.
func (s *Source) discoverRealtimeDatabases(ctx context.Context, projectDetails map[string]any) (discovered, error) {
	service, err := s.realtimeDatabaseService(ctx)
	if err != nil {
		return discovered{}, err
	}

	callCtx, cancel := context.WithTimeout(ctx, s.callTimeout())
	defer cancel()

	var instances []*firebasedatabase.DatabaseInstance
	pages := 0

	// "-" as the location asks for instances in every location at once.
	err = service.Projects.Locations.Instances.
		List(fmt.Sprintf("projects/%s/locations/-", s.config.ProjectID)).
		Pages(callCtx, func(response *firebasedatabase.ListDatabaseInstancesResponse) error {
			instances = append(instances, response.Instances...)
			pages++
			if pages >= maxInstancePages {
				log.Warn().Int("pages", pages).Msg("Stopped listing Realtime Database instances at the page cap")
				return errPageCap
			}
			return nil
		})
	if err != nil && !errors.Is(err, errPageCap) {
		return discovered{}, fmt.Errorf("listing Realtime Database instances: %w", err)
	}

	var result discovered

	for _, instance := range instances {
		instanceID := lastSegment(instance.Name)

		metadata := withProjectDetails(projectDetails)
		metadata["instance_id"] = instanceID
		metadata["database_kind"] = "realtime"
		setIfNotEmpty(metadata, "database_url", instance.DatabaseUrl)
		setIfNotEmpty(metadata, "instance_type", instance.Type)
		setIfNotEmpty(metadata, "state", instance.State)

		name := realtimeDatabaseName(instanceID)
		instanceMRN := assetMRN("Database", name)
		description := fmt.Sprintf("Firebase Realtime Database instance %s", instanceID)

		asset := pluginsdk.Asset{
			Name:        &name,
			MRN:         &instanceMRN,
			Type:        "Database",
			Providers:   []string{provider},
			Description: &description,
			Metadata:    metadata,
			Tags:        pluginsdk.InterpolateTags(s.config.Tags, metadata),
			Sources: []pluginsdk.AssetSource{{
				Name:       provider,
				LastSyncAt: time.Now(),
				Properties: metadata,
				Priority:   1,
			}},
		}

		if instance.DatabaseUrl != "" {
			asset.ExternalLinks = []pluginsdk.AssetExternalLink{{
				Name: "Open database URL",
				URL:  instance.DatabaseUrl,
			}}
		}

		result.assets = append(result.assets, asset)
	}

	log.Debug().Int("count", len(result.assets)).Msg("Discovered Realtime Database instances")

	return result, nil
}

// projectDetails reads the project's name and number. It decorates the assets
// rather than being the point of the run, so a failure is a warning and the
// fields are simply missing.
func (s *Source) projectDetails(ctx context.Context) map[string]any {
	if !s.config.IncludeProjectDetails {
		return nil
	}

	service, err := s.firebaseService(ctx)
	if err != nil {
		log.Warn().Err(err).Msg("Failed to create Firebase client, project details are missing from this run")
		return nil
	}

	callCtx, cancel := context.WithTimeout(ctx, s.callTimeout())
	defer cancel()

	project, err := service.Projects.
		Get("projects/" + s.config.ProjectID).
		Context(callCtx).
		Do()
	if err != nil {
		log.Warn().Err(err).Msg("Failed to read Firebase project details")
		return nil
	}

	details := make(map[string]any, 3)
	setIfNotEmpty(details, "firebase_project_id", project.ProjectId)
	setIfNotEmpty(details, "firebase_project_display_name", project.DisplayName)
	if project.ProjectNumber != 0 {
		details["firebase_project_number"] = project.ProjectNumber
	}

	return details
}
