package firebase

import (
	"context"
	"fmt"
	"sort"
	"time"

	"cloud.google.com/go/firestore"
	pluginsdk "github.com/marmotdata/plugin-sdk"
	"github.com/rs/zerolog/log"
	firestoreadmin "google.golang.org/api/firestore/v1"
)

// firestoreDatabase is one database to scan. details is nil when the database
// id came from the config rather than the admin API, which is how the plugin
// runs against the Firestore emulator: the emulator serves documents but not
// the admin API that lists databases.
type firestoreDatabase struct {
	id      string
	details *firestoreadmin.GoogleFirestoreAdminV1Database
}

// collectionGroup is one catalogued collection: the path it is stored under
// with document ids left out, plus every concrete collection in the database
// that path stands for.
//
// A subcollection lives under a document, so "lines" under "orders" really
// exists once per order. Those all share a shape, so they become one asset
// named "orders/lines" and the sampling reads across all of them.
type collectionGroup struct {
	id        string
	path      string
	parent    string
	parentMRN string
	depth     int
	refs      []*firestore.CollectionRef
}

func (s *Source) discoverFirestore(ctx context.Context, projectDetails map[string]any) (discovered, error) {
	databases, err := s.firestoreDatabases(ctx)
	if err != nil {
		return discovered{}, err
	}

	var result discovered

	for _, database := range databases {
		metadata := withProjectDetails(projectDetails)
		metadata["database_id"] = database.id
		metadata["database_kind"] = "firestore"
		addFirestoreDatabaseMetadata(metadata, database.details)

		name := firestoreDatabaseName(database.id)
		databaseMRN := assetMRN("Database", name)
		description := fmt.Sprintf("Firestore database %s", database.id)

		result.assets = append(result.assets, pluginsdk.Asset{
			Name:        &name,
			MRN:         &databaseMRN,
			Type:        "Database",
			Providers:   []string{provider},
			Description: &description,
			Metadata:    metadata,
			Tags:        pluginsdk.InterpolateTags(s.config.Tags, metadata),
			ExternalLinks: []pluginsdk.AssetExternalLink{{
				Name: "Open in Firebase",
				URL:  fmt.Sprintf("https://console.firebase.google.com/project/%s/firestore", s.config.ProjectID),
			}},
			Sources: []pluginsdk.AssetSource{{
				Name:       provider,
				LastSyncAt: time.Now(),
				Properties: metadata,
				Priority:   1,
			}},
		})

		collections, err := s.discoverCollections(ctx, database, databaseMRN, projectDetails)
		if err != nil {
			log.Warn().Err(err).Str("database", database.id).Msg("Failed to discover collections")
			continue
		}
		result.merge(collections)

		log.Debug().Str("database", database.id).Int("count", len(collections.assets)).Msg("Discovered collections")
	}

	return result, nil
}

// firestoreDatabases lists the databases to scan. Configured ids skip the
// admin API entirely, which is what lets the plugin run against an emulator.
func (s *Source) firestoreDatabases(ctx context.Context) ([]firestoreDatabase, error) {
	if len(s.config.Databases) > 0 {
		databases := make([]firestoreDatabase, 0, len(s.config.Databases))
		for _, id := range s.config.Databases {
			databases = append(databases, firestoreDatabase{id: id})
		}
		return databases, nil
	}

	service, err := s.firestoreAdminService(ctx)
	if err != nil {
		return nil, err
	}

	callCtx, cancel := context.WithTimeout(ctx, s.callTimeout())
	defer cancel()

	response, err := service.Projects.Databases.
		List("projects/" + s.config.ProjectID).
		Context(callCtx).
		Do()
	if err != nil {
		return nil, fmt.Errorf("listing Firestore databases: %w", err)
	}

	for _, location := range response.Unreachable {
		log.Warn().Str("location", location).Msg("Firestore location unreachable, its databases are missing from this run")
	}

	databases := make([]firestoreDatabase, 0, len(response.Databases))
	for _, details := range response.Databases {
		databases = append(databases, firestoreDatabase{id: lastSegment(details.Name), details: details})
	}
	return databases, nil
}

func addFirestoreDatabaseMetadata(metadata map[string]any, details *firestoreadmin.GoogleFirestoreAdminV1Database) {
	if details == nil {
		return
	}

	setIfNotEmpty(metadata, "location_id", details.LocationId)
	setIfNotEmpty(metadata, "database_type", details.Type)
	setIfNotEmpty(metadata, "concurrency_mode", details.ConcurrencyMode)
	setIfNotEmpty(metadata, "point_in_time_recovery", details.PointInTimeRecoveryEnablement)
	setIfNotEmpty(metadata, "delete_protection", details.DeleteProtectionState)
	setIfNotEmpty(metadata, "version_retention_period", details.VersionRetentionPeriod)
	setIfNotEmpty(metadata, "earliest_version_time", details.EarliestVersionTime)
	setIfNotEmpty(metadata, "create_time", details.CreateTime)
	setIfNotEmpty(metadata, "update_time", details.UpdateTime)
	setIfNotEmpty(metadata, "uid", details.Uid)
	metadata["free_tier"] = details.FreeTier
}

// discoverCollections walks the database breadth first, one level of
// subcollections at a time, up to max_collection_depth.
func (s *Source) discoverCollections(ctx context.Context, database firestoreDatabase, databaseMRN string, projectDetails map[string]any) (discovered, error) {
	client, err := s.firestoreClient(ctx, database.id)
	if err != nil {
		return discovered{}, err
	}
	defer client.Close()

	listCtx, cancel := context.WithTimeout(ctx, s.callTimeout())
	roots, err := client.Collections(listCtx).GetAll()
	cancel()
	if err != nil {
		return discovered{}, fmt.Errorf("listing root collections: %w", err)
	}

	queue := make([]collectionGroup, 0, len(roots))
	for _, ref := range roots {
		queue = append(queue, collectionGroup{
			id:        ref.ID,
			path:      ref.ID,
			parentMRN: databaseMRN,
			depth:     1,
			refs:      []*firestore.CollectionRef{ref},
		})
	}
	sortGroups(queue)

	var result discovered

	for len(queue) > 0 {
		group := queue[0]
		queue = queue[1:]

		snapshots := s.sampleDocuments(ctx, group)

		documents := make([]map[string]any, 0, len(snapshots))
		for _, snapshot := range snapshots {
			documents = append(documents, snapshot.Data())
		}
		columns := inferColumns(documents)

		asset, err := s.collectionAsset(database.id, group, len(snapshots), columns, projectDetails)
		if err != nil {
			log.Warn().Err(err).Str("collection", group.path).Msg("Failed to build collection asset")
			continue
		}

		result.assets = append(result.assets, asset)
		result.lineage = append(result.lineage, pluginsdk.LineageEdge{
			Source: group.parentMRN,
			Target: *asset.MRN,
			Type:   "CONTAINS",
		})
		result.statistics = append(result.statistics, pluginsdk.Statistic{
			AssetMRN:   *asset.MRN,
			MetricName: "asset.column_count",
			Value:      float64(len(columns)),
		})

		if !s.config.IncludeSubcollections || group.depth >= s.config.MaxCollectionDepth {
			continue
		}
		queue = append(queue, s.subcollectionGroups(ctx, group, *asset.MRN, snapshots)...)
	}

	return result, nil
}

func (s *Source) collectionAsset(databaseID string, group collectionGroup, sampled int, columns []pluginsdk.Column, projectDetails map[string]any) (pluginsdk.Asset, error) {
	metadata := withProjectDetails(projectDetails)
	metadata["database_id"] = databaseID
	metadata["collection_id"] = group.id
	metadata["collection_path"] = group.path
	metadata["depth"] = group.depth
	metadata["sampled_documents"] = sampled
	if group.parent != "" {
		// collection_group_id is the id a Firestore collection group query
		// takes, which is how a reader reaches every copy of this
		// subcollection across its parent documents.
		metadata["collection_group_id"] = group.id
		metadata["parent_path"] = group.parent
	}

	name := collectionName(databaseID, group.path)
	collectionMRN := assetMRN("Collection", name)
	description := fmt.Sprintf("Firestore collection %s in database %s", group.path, databaseID)

	asset := pluginsdk.Asset{
		Name:        &name,
		MRN:         &collectionMRN,
		Type:        "Collection",
		Providers:   []string{provider},
		Description: &description,
		Metadata:    metadata,
		Schema:      make(map[string]string),
		Tags:        pluginsdk.InterpolateTags(s.config.Tags, metadata),
		Sources: []pluginsdk.AssetSource{{
			Name:       provider,
			LastSyncAt: time.Now(),
			Properties: metadata,
			Priority:   1,
		}},
	}

	if err := pluginsdk.SetColumns(&asset, columns); err != nil {
		return pluginsdk.Asset{}, fmt.Errorf("setting columns on %s: %w", name, err)
	}

	return asset, nil
}

// sampleDocuments reads up to sample_documents documents, spreading the
// budget over the concrete collections the group stands for so a
// subcollection's shape is drawn from more than one parent document.
func (s *Source) sampleDocuments(ctx context.Context, group collectionGroup) []*firestore.DocumentSnapshot {
	var snapshots []*firestore.DocumentSnapshot

	for _, ref := range group.refs {
		remaining := s.config.SampleDocuments - len(snapshots)
		if remaining <= 0 {
			break
		}

		queryCtx, cancel := context.WithTimeout(ctx, s.callTimeout())
		documents, err := ref.Limit(remaining).Documents(queryCtx).GetAll()
		cancel()
		if err != nil {
			log.Warn().Err(err).Str("collection", ref.Path).Msg("Failed to sample documents")
			continue
		}

		snapshots = append(snapshots, documents...)
	}

	return snapshots
}

// subcollectionGroups groups the subcollections of the sampled documents by
// their id, so the many "lines" collections under many orders become one
// group to catalogue.
func (s *Source) subcollectionGroups(ctx context.Context, parent collectionGroup, parentMRN string, snapshots []*firestore.DocumentSnapshot) []collectionGroup {
	byID := make(map[string][]*firestore.CollectionRef)

	for _, snapshot := range snapshots {
		listCtx, cancel := context.WithTimeout(ctx, s.callTimeout())
		children, err := snapshot.Ref.Collections(listCtx).GetAll()
		cancel()
		if err != nil {
			log.Warn().Err(err).Str("document", snapshot.Ref.Path).Msg("Failed to list subcollections")
			continue
		}

		for _, child := range children {
			byID[child.ID] = append(byID[child.ID], child)
		}
	}

	groups := make([]collectionGroup, 0, len(byID))
	for id, refs := range byID {
		groups = append(groups, collectionGroup{
			id:        id,
			path:      parent.path + "/" + id,
			parent:    parent.path,
			parentMRN: parentMRN,
			depth:     parent.depth + 1,
			refs:      refs,
		})
	}
	sortGroups(groups)

	return groups
}

// sortGroups keeps the asset order the same between runs, which map iteration
// alone would not.
func sortGroups(groups []collectionGroup) {
	sort.Slice(groups, func(i, j int) bool { return groups[i].path < groups[j].path })
}
