// Package firebase discovers Firestore databases and collections, and
// Realtime Database instances, from a Firebase project.
package firebase

import (
	"context"
	"fmt"
	"strings"
	"time"

	pluginsdk "github.com/marmotdata/plugin-sdk"
	"github.com/marmotdata/plugin-sdk/mrn"
	"github.com/rs/zerolog/log"
)

// provider is the exact service string every Firebase asset and every MRN
// this plugin builds carries.
const provider = "Firebase"

// Config for the Firebase plugin.
type Config struct {
	pluginsdk.BaseConfig `json:",inline"`

	ProjectID       string   `json:"project_id" label:"Project ID" description:"Google Cloud project ID" validate:"required"`
	Databases       []string `json:"databases,omitempty" description:"Firestore database IDs to scan. Every database is listed from the API when this is empty. The Firestore emulator has no such API, so name the databases here when pointing at one"`
	CredentialsFile string   `json:"credentials_file,omitempty" description:"Path to service account JSON file"`
	CredentialsJSON string   `json:"credentials_json,omitempty" description:"Service account JSON content" sensitive:"true"`
	Endpoint        string   `json:"endpoint,omitempty" description:"Custom endpoint URL, for testing against a local server"`
	DisableAuth     bool     `json:"disable_auth,omitempty" description:"Disable authentication, for local testing"`

	IncludeRealtimeDatabase bool `json:"include_realtime_database" description:"Whether to discover Realtime Database instances" default:"true"`
	IncludeProjectDetails   bool `json:"include_project_details" description:"Whether to read the Firebase project name and number" default:"true"`
	IncludeSubcollections   bool `json:"include_subcollections" description:"Whether to descend into subcollections" default:"true"`
	MaxCollectionDepth      int  `json:"max_collection_depth" description:"How many levels of subcollection to descend" default:"2" validate:"omitempty,min=1,max=5"`
	SampleDocuments         int  `json:"sample_documents" description:"How many documents to read per collection to infer its fields" default:"20" validate:"omitempty,min=1,max=500"`
}

// Example configuration for the plugin
var _ = `
project_id: "my-firebase-project"
credentials_file: "/path/to/service-account.json"
sample_documents: 50
max_collection_depth: 2
tags:
  - "firebase"
  - "firestore"
`

// Meta describes the plugin to the Marmot host.
func Meta() pluginsdk.Meta {
	return pluginsdk.Meta{
		ID:          "firebase",
		Name:        "Firebase",
		Description: "Discover Firestore collections and Realtime Database instances from Firebase projects",
		Icon:        "firebase",
		Category:    "database",
		Status:      "experimental",
		// Discover emits CONTAINS edges from databases to collections and
		// from collections to their subcollections.
		Features:   []string{"Assets", "Lineage"},
		ConfigSpec: pluginsdk.GenerateConfigSpec(Config{}),
	}
}

// defaultCallTimeout bounds a single call to a Firebase API.
const defaultCallTimeout = 30 * time.Second

// Source represents the Firebase plugin.
type Source struct {
	config *Config
	// timeout bounds one API call. Zero means defaultCallTimeout; the tests
	// set it short so a Firestore endpoint that cannot answer does not hold
	// a test open for the whole production timeout.
	timeout time.Duration
}

func (s *Source) callTimeout() time.Duration {
	if s.timeout > 0 {
		return s.timeout
	}
	return defaultCallTimeout
}

// discovered is what one part of a project contributed. The Firestore pass
// and the Realtime Database pass each build one, so either can fail without
// losing the other's assets.
type discovered struct {
	assets     []pluginsdk.Asset
	lineage    []pluginsdk.LineageEdge
	statistics []pluginsdk.Statistic
}

func (d *discovered) merge(other discovered) {
	d.assets = append(d.assets, other.assets...)
	d.lineage = append(d.lineage, other.lineage...)
	d.statistics = append(d.statistics, other.statistics...)
}

// Validate validates and normalises the plugin configuration.
func (s *Source) Validate(rawConfig pluginsdk.RawConfig) (pluginsdk.RawConfig, error) {
	config, err := pluginsdk.UnmarshalConfig[Config](rawConfig)
	if err != nil {
		return nil, fmt.Errorf("unmarshalling config: %w", err)
	}

	pluginsdk.ApplyDefaults(config, rawConfig)

	// The generated Google API clients append their paths to the endpoint as
	// a relative URL reference, which drops the endpoint's last path segment
	// unless it ends in a slash. Their own default endpoints end in one.
	if config.Endpoint != "" && !strings.HasSuffix(config.Endpoint, "/") {
		config.Endpoint += "/"
	}

	if err := pluginsdk.ValidateStruct(config); err != nil {
		return nil, err
	}

	s.config = config
	return rawConfig, nil
}

// Discover discovers Firestore databases and collections, and Realtime
// Database instances.
func (s *Source) Discover(ctx context.Context, rawConfig pluginsdk.RawConfig) (*pluginsdk.DiscoveryResult, error) {
	// The host spawns a fresh plugin process per call, so Discover cannot
	// rely on state set by an earlier Validate call.
	if _, err := s.Validate(rawConfig); err != nil {
		return nil, fmt.Errorf("validating config: %w", err)
	}

	ctx, cancel := context.WithTimeout(ctx, 10*time.Minute)
	defer cancel()

	projectDetails := s.projectDetails(ctx)

	var result discovered
	var firstErr error

	firestoreResult, err := s.discoverFirestore(ctx, projectDetails)
	if err != nil {
		log.Warn().Err(err).Msg("Failed to discover Firestore databases")
		firstErr = err
	}
	result.merge(firestoreResult)

	if s.config.IncludeRealtimeDatabase {
		realtimeResult, err := s.discoverRealtimeDatabases(ctx, projectDetails)
		if err != nil {
			log.Warn().Err(err).Msg("Failed to discover Realtime Database instances")
			if firstErr == nil {
				firstErr = err
			}
		}
		result.merge(realtimeResult)
	}

	// One broken part of a project must not cost the rest, but a run that
	// discovered nothing at all and hit an error means the project is
	// unreachable, and that is worth failing on.
	if len(result.assets) == 0 && firstErr != nil {
		return nil, firstErr
	}

	log.Info().
		Int("assets", len(result.assets)).
		Int("lineages", len(result.lineage)).
		Int("statistics", len(result.statistics)).
		Msg("Firebase discovery completed")

	return &pluginsdk.DiscoveryResult{
		Assets:     result.assets,
		Lineage:    result.lineage,
		Statistics: result.statistics,
	}, nil
}

// assetMRN is the single place a Firebase MRN is built. Assets and both ends
// of every lineage edge go through it, so the two can never drift into
// addressing the same object differently.
func assetMRN(assetType, name string) string {
	return mrn.New(assetType, provider, name)
}

// firestoreDatabaseName names a Firestore database asset. A Firestore
// database id and a Realtime Database instance id can be the same string, so
// the kind is part of the name to keep the two apart.
func firestoreDatabaseName(databaseID string) string {
	return "firestore/" + databaseID
}

// realtimeDatabaseName names a Realtime Database instance asset.
func realtimeDatabaseName(instanceID string) string {
	return "rtdb/" + instanceID
}

// collectionName names a Firestore collection asset. path is the collection
// path with document ids left out, so "orders/lines" rather than
// "orders/<some order id>/lines".
func collectionName(databaseID, path string) string {
	return firestoreDatabaseName(databaseID) + "/" + path
}

// withProjectDetails copies the project-level fields onto a fresh metadata
// map. Every asset gets its own map because it is also handed to the asset's
// Sources entry.
func withProjectDetails(projectDetails map[string]any) map[string]any {
	metadata := make(map[string]any, len(projectDetails)+8)
	for key, value := range projectDetails {
		metadata[key] = value
	}
	return metadata
}

// setIfNotEmpty keeps empty strings out of the metadata map, so a field the
// API did not return does not show up in the catalog as a blank value.
func setIfNotEmpty(metadata map[string]any, key, value string) {
	if value != "" {
		metadata[key] = value
	}
}

// lastSegment returns the id at the end of a Google resource name, for
// example "orders" from "projects/p/databases/(default)/documents/orders".
func lastSegment(resourceName string) string {
	if idx := strings.LastIndex(resourceName, "/"); idx >= 0 {
		return resourceName[idx+1:]
	}
	return resourceName
}
