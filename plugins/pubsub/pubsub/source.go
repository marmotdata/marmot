// Package pubsub discovers topics and subscriptions from Google Cloud Pub/Sub.
package pubsub

import (
	"context"
	"fmt"
	"strings"
	"time"

	pluginsdk "github.com/marmotdata/plugin-sdk"
	"github.com/marmotdata/plugin-sdk/mrn"
	"github.com/rs/zerolog/log"
)

// provider is the exact service string Marmot stores for Pub/Sub assets. It
// matches the projection the OpenMetadata plugin uses, so both plugins
// address the same topic with the same MRN.
const provider = "GooglePubSub"

// Delivery types a subscription can have. Pub/Sub allows exactly one of push,
// BigQuery or Cloud Storage; with none of them set the subscriber pulls.
const (
	deliveryPull         = "pull"
	deliveryPush         = "push"
	deliveryBigQuery     = "bigquery"
	deliveryCloudStorage = "cloud_storage"
)

// Sampling reads a small, bounded batch so an asset preview never turns into
// a long-running consumer.
const (
	sampleMessageLimit = 20
	sampleMessageWait  = 5 * time.Second
)

// Config for the Google Pub/Sub plugin.
type Config struct {
	pluginsdk.BaseConfig `json:",inline"`

	ProjectID       string `json:"project_id" label:"Project ID" description:"Google Cloud project ID" validate:"required"`
	EmulatorHost    string `json:"emulator_host,omitempty" label:"Emulator Host" description:"Address of a Pub/Sub emulator, for example localhost:8085"`
	CredentialsFile string `json:"credentials_file,omitempty" description:"Path to a service account JSON file"`
	CredentialsJSON string `json:"credentials_json,omitempty" description:"Service account JSON content" sensitive:"true"`

	IncludeSubscriptions    bool `json:"include_subscriptions" description:"Whether to discover subscriptions" default:"true"`
	IncludeSchemas          bool `json:"include_schemas" description:"Whether to attach topic schemas and their fields" default:"true"`
	IncludeDeadLetterTopics bool `json:"include_dead_letter_topics" description:"Whether to record dead letter topics on subscriptions" default:"true"`
	IncludeSampleMessages   bool `json:"include_sample_messages" description:"Whether to allow reading sample messages for asset previews" default:"false"`
}

// Example configuration for the plugin
var _ = `
project_id: "company-streaming"
credentials_file: "/etc/marmot/pubsub-service-account.json"
include_subscriptions: true
include_schemas: true
tags:
  - "pubsub"
  - "streaming"
`

// Meta describes the plugin to the Marmot host.
func Meta() pluginsdk.Meta {
	return pluginsdk.Meta{
		ID:          "pubsub",
		Name:        "Google Pub/Sub",
		Description: "Discover topics and subscriptions from Google Cloud Pub/Sub",
		Icon:        "googlepubsub",
		Category:    "messaging",
		Status:      "experimental",
		// Discover links topics to subscriptions and subscriptions to their
		// export destinations, so the manifest declares Lineage too.
		Features:   []string{"Assets", "Lineage"},
		ConfigSpec: pluginsdk.GenerateConfigSpec(Config{}),
	}
}

// Source represents the Google Pub/Sub plugin.
type Source struct {
	config *Config
	// newClient is swapped in tests to run discovery against a fake project.
	newClient func(ctx context.Context, config *Config) (client, error)
}

// Validate validates and normalises the plugin configuration.
func (s *Source) Validate(rawConfig pluginsdk.RawConfig) (pluginsdk.RawConfig, error) {
	config, err := pluginsdk.UnmarshalConfig[Config](rawConfig)
	if err != nil {
		return nil, fmt.Errorf("unmarshalling config: %w", err)
	}

	pluginsdk.ApplyDefaults(config, rawConfig)

	// An emulator address is a host:port, but people paste a URL, so strip a
	// scheme rather than failing on it.
	config.EmulatorHost = strings.TrimSuffix(config.EmulatorHost, "/")
	config.EmulatorHost = strings.TrimPrefix(config.EmulatorHost, "http://")
	config.EmulatorHost = strings.TrimPrefix(config.EmulatorHost, "https://")

	if config.CredentialsFile != "" && config.CredentialsJSON != "" {
		return nil, fmt.Errorf("only one of credentials_file or credentials_json may be set")
	}

	if err := pluginsdk.ValidateStruct(config); err != nil {
		return nil, err
	}

	s.config = config
	return rawConfig, nil
}

// Discover discovers Pub/Sub topics, subscriptions and the links between them.
func (s *Source) Discover(ctx context.Context, rawConfig pluginsdk.RawConfig) (*pluginsdk.DiscoveryResult, error) {
	// The host spawns a fresh plugin process per call, so Discover cannot
	// rely on state set by an earlier Validate call.
	if _, err := s.Validate(rawConfig); err != nil {
		return nil, fmt.Errorf("validating config: %w", err)
	}

	ctx, cancel := context.WithTimeout(ctx, 10*time.Minute)
	defer cancel()

	c, err := s.connect(ctx)
	if err != nil {
		return nil, fmt.Errorf("connecting to Pub/Sub: %w", err)
	}
	defer c.Close()

	return s.discover(ctx, c)
}

// discover holds the discovery logic that does not depend on how the client
// was built, so tests can drive it with a fake project.
func (s *Source) discover(ctx context.Context, c client) (*pluginsdk.DiscoveryResult, error) {
	topics, err := c.Topics(ctx)
	if err != nil {
		return nil, fmt.Errorf("listing topics: %w", err)
	}
	log.Debug().Int("count", len(topics)).Msg("Listed Pub/Sub topics")

	schemas := s.loadSchemas(ctx, c)

	var assets []pluginsdk.Asset
	var lineages []pluginsdk.LineageEdge

	for _, topic := range topics {
		var subs []subscriptionInfo
		if s.config.IncludeSubscriptions {
			subs, err = c.Subscriptions(ctx, topic.ID)
			if err != nil {
				// One unreadable topic must not lose the rest of the project.
				log.Warn().Err(err).Str("topic", topic.ID).Msg("Failed to list subscriptions")
			}
		}

		assets = append(assets, s.topicAsset(topic, subs, schemas))
		lineages = append(lineages, s.topicIngestionLineage(topic)...)

		for _, sub := range subs {
			assets = append(assets, s.subscriptionAsset(sub))
			lineages = append(lineages, s.subscriptionLineage(topic, sub)...)
		}
	}

	log.Info().
		Int("assets", len(assets)).
		Int("lineages", len(lineages)).
		Msg("Pub/Sub discovery completed")

	return &pluginsdk.DiscoveryResult{
		Assets:  assets,
		Lineage: lineages,
	}, nil
}

// loadSchemas returns every schema in the project keyed by its full resource
// name, which is how a topic refers to one. Schemas are optional: the Pub/Sub
// emulator has no schema service, and a service account may lack the viewer
// role, so a failure only drops the schema details.
func (s *Source) loadSchemas(ctx context.Context, c client) map[string]schemaInfo {
	if !s.config.IncludeSchemas {
		return nil
	}

	list, err := c.Schemas(ctx)
	if err != nil {
		log.Warn().Err(err).Msg("Failed to list schemas, continuing without them")
		return nil
	}

	byName := make(map[string]schemaInfo, len(list))
	for _, schema := range list {
		byName[schema.FullName] = schema
	}
	return byName
}

func (s *Source) topicAsset(topic topicInfo, subs []subscriptionInfo, schemas map[string]schemaInfo) pluginsdk.Asset {
	metadata := map[string]any{
		"project_id": s.config.ProjectID,
		"topic_name": topic.FullName,
	}

	// With subscriptions turned off none were listed, and a count of zero
	// would read as "this topic has no subscribers".
	if s.config.IncludeSubscriptions {
		metadata["subscription_count"] = len(subs)
	}

	if len(topic.Labels) > 0 {
		metadata["labels"] = topic.Labels
	}
	if topic.KMSKeyName != "" {
		metadata["kms_key_name"] = topic.KMSKeyName
	}
	if topic.RetentionDuration > 0 {
		metadata["retention"] = topic.RetentionDuration.String()
	}
	if len(topic.AllowedPersistenceRegions) > 0 {
		metadata["allowed_persistence_regions"] = topic.AllowedPersistenceRegions
	}
	if topic.State != "" {
		metadata["state"] = topic.State
	}
	if topic.IngestionSource != "" {
		metadata["ingestion_source"] = topic.IngestionSource
	}

	if ids := subscriptionIDs(subs); len(ids) > 0 {
		metadata["subscriptions"] = ids
	}

	schema, hasSchema := schemas[topic.SchemaName]
	if topic.SchemaName != "" {
		metadata["schema"] = lastSegment(topic.SchemaName)
		if topic.SchemaEncoding != "" {
			metadata["schema_encoding"] = topic.SchemaEncoding
		}
		if hasSchema {
			metadata["schema_type"] = schema.Type
			if schema.RevisionID != "" {
				metadata["schema_revision"] = schema.RevisionID
			}
		}
	}

	url := s.topicURL(topic.ID)
	if url != "" {
		metadata["url"] = url
	}

	name := topic.ID
	mrnValue := assetMRN("Topic", name)

	asset := pluginsdk.Asset{
		Name:      &name,
		MRN:       &mrnValue,
		Type:      "Topic",
		Providers: []string{provider},
		Metadata:  metadata,
		Schema:    make(map[string]string),
		Tags:      pluginsdk.InterpolateTags(s.config.Tags, metadata),
		Sources: []pluginsdk.AssetSource{{
			Name:       provider,
			LastSyncAt: time.Now(),
			Properties: metadata,
			Priority:   1,
		}},
	}

	if url != "" {
		asset.ExternalLinks = []pluginsdk.AssetExternalLink{{
			Name: "Open in Google Cloud Console",
			URL:  url,
		}}
	}

	if hasSchema && schema.Definition != "" {
		// The raw definition is kept whatever the schema type, because a
		// protobuf definition cannot be turned into a column list.
		asset.Schema["schema"] = schema.Definition

		if schema.Type == "AVRO" {
			columns, err := avroColumns(schema.Definition)
			if err != nil {
				log.Warn().Err(err).Str("topic", topic.ID).Msg("Failed to parse Avro schema")
			} else if len(columns) > 0 {
				if err := pluginsdk.SetColumns(&asset, columns); err != nil {
					log.Warn().Err(err).Str("topic", topic.ID).Msg("Failed to set schema columns")
				}
			}
		}
	}

	return asset
}

func (s *Source) subscriptionAsset(sub subscriptionInfo) pluginsdk.Asset {
	metadata := map[string]any{
		"project_id":              s.config.ProjectID,
		"subscription_name":       sub.FullName,
		"topic":                   sub.TopicID,
		"delivery_type":           deliveryType(sub),
		"ack_deadline_seconds":    int(sub.AckDeadline.Seconds()),
		"retain_acked_messages":   sub.RetainAckedMessages,
		"enable_message_ordering": sub.EnableMessageOrdering,
		"exactly_once_delivery":   sub.ExactlyOnceDelivery,
		"detached":                sub.Detached,
	}

	if sub.PushEndpoint != "" {
		metadata["push_endpoint"] = sub.PushEndpoint
	}
	if sub.BigQueryTable != "" {
		metadata["bigquery_table"] = sub.BigQueryTable
		metadata["bigquery_use_topic_schema"] = sub.BigQueryUseTopicSchema
		if sub.BigQueryState != "" {
			metadata["bigquery_state"] = sub.BigQueryState
		}
	}
	if sub.CloudStorageBucket != "" {
		metadata["cloud_storage_bucket"] = sub.CloudStorageBucket
		if sub.CloudStorageFilenamePrefix != "" {
			metadata["cloud_storage_filename_prefix"] = sub.CloudStorageFilenamePrefix
		}
		if sub.CloudStorageFilenameSuffix != "" {
			metadata["cloud_storage_filename_suffix"] = sub.CloudStorageFilenameSuffix
		}
		if sub.CloudStorageState != "" {
			metadata["cloud_storage_state"] = sub.CloudStorageState
		}
	}
	if sub.RetentionDuration > 0 {
		metadata["message_retention"] = sub.RetentionDuration.String()
	}
	if sub.ExpirationTTL > 0 {
		metadata["expiration_ttl"] = sub.ExpirationTTL.String()
	}
	if sub.Filter != "" {
		metadata["filter"] = sub.Filter
	}
	if len(sub.Labels) > 0 {
		metadata["labels"] = sub.Labels
	}
	if sub.State != "" {
		metadata["state"] = sub.State
	}
	if s.config.IncludeDeadLetterTopics && sub.DeadLetterTopicID != "" {
		metadata["dead_letter_topic"] = sub.DeadLetterTopicID
		metadata["max_delivery_attempts"] = sub.MaxDeliveryAttempts
	}

	url := s.subscriptionURL(sub.ID)
	if url != "" {
		metadata["url"] = url
	}

	name := sub.ID
	mrnValue := assetMRN("Subscription", name)

	asset := pluginsdk.Asset{
		Name:      &name,
		MRN:       &mrnValue,
		Type:      "Subscription",
		Providers: []string{provider},
		Metadata:  metadata,
		Schema:    make(map[string]string),
		Tags:      pluginsdk.InterpolateTags(s.config.Tags, metadata),
		Sources: []pluginsdk.AssetSource{{
			Name:       provider,
			LastSyncAt: time.Now(),
			Properties: metadata,
			Priority:   1,
		}},
	}

	if url != "" {
		asset.ExternalLinks = []pluginsdk.AssetExternalLink{{
			Name: "Open in Google Cloud Console",
			URL:  url,
		}}
	}

	return asset
}

// subscriptionLineage links a subscription to the topic it reads and to
// wherever its messages end up.
func (s *Source) subscriptionLineage(topic topicInfo, sub subscriptionInfo) []pluginsdk.LineageEdge {
	subMRN := assetMRN("Subscription", sub.ID)

	edges := []pluginsdk.LineageEdge{{
		Source: assetMRN("Topic", topic.ID),
		Target: subMRN,
		Type:   "FEEDS",
	}}

	// BigQuery and Cloud Storage subscriptions write straight into another
	// system. Those assets belong to the BigQuery and GCS plugins, so only
	// the edge is emitted; Marmot drops it when the target is not catalogued.
	if table := bigQueryTableID(sub.BigQueryTable); table != "" {
		edges = append(edges, pluginsdk.LineageEdge{
			Source: subMRN,
			Target: mrn.New("Table", "BigQuery", table),
			Type:   "PRODUCES",
		})
	}

	if sub.CloudStorageBucket != "" {
		edges = append(edges, pluginsdk.LineageEdge{
			Source: subMRN,
			Target: mrn.New("Bucket", "GCS", sub.CloudStorageBucket),
			Type:   "PRODUCES",
		})
	}

	if s.config.IncludeDeadLetterTopics && sub.DeadLetterTopicID != "" {
		edges = append(edges, pluginsdk.LineageEdge{
			Source: subMRN,
			Target: assetMRN("Topic", sub.DeadLetterTopicID),
			Type:   "FEEDS",
		})
	}

	return edges
}

// topicIngestionLineage links a topic to the external stream Pub/Sub imports
// messages from, when the topic has an ingestion source Marmot can name.
func (s *Source) topicIngestionLineage(topic topicInfo) []pluginsdk.LineageEdge {
	stream := kinesisStreamName(topic.IngestionKinesisStreamARN)
	if stream == "" {
		return nil
	}

	return []pluginsdk.LineageEdge{{
		Source: mrn.New("Stream", "Kinesis", stream),
		Target: assetMRN("Topic", topic.ID),
		Type:   "FEEDS",
	}}
}

// FetchSampleData reads a handful of live messages so the UI can preview what
// flows through a topic or subscription. Every message is nacked, so it is
// redelivered to the real consumer straight after; the preview is a read, but
// not a free one, which is why it is off by default.
func (s *Source) FetchSampleData(ctx context.Context, rawConfig pluginsdk.RawConfig, a *pluginsdk.Asset) ([]string, [][]any, error) {
	if a == nil || a.Name == nil {
		return nil, nil, fmt.Errorf("asset is nil")
	}

	if _, err := s.Validate(rawConfig); err != nil {
		return nil, nil, fmt.Errorf("validating config: %w", err)
	}

	if !s.config.IncludeSampleMessages {
		return nil, nil, fmt.Errorf("sample messages are disabled, set include_sample_messages to true")
	}

	fetchCtx, cancel := context.WithTimeout(ctx, 2*sampleMessageWait)
	defer cancel()

	c, err := s.connect(fetchCtx)
	if err != nil {
		return nil, nil, fmt.Errorf("connecting to Pub/Sub: %w", err)
	}
	defer c.Close()

	subscriptionID, err := sampleSubscriptionID(fetchCtx, c, a)
	if err != nil {
		return nil, nil, err
	}

	messages, err := c.ReceiveMessages(fetchCtx, subscriptionID, sampleMessageLimit, sampleMessageWait)
	if err != nil {
		return nil, nil, err
	}

	return sampleRows(messages)
}

// sampleSubscriptionID picks the subscription to read from: a Subscription
// asset is read directly, a Topic asset borrows one of its pull
// subscriptions, since push, BigQuery and Cloud Storage subscriptions cannot
// be pulled from.
func sampleSubscriptionID(ctx context.Context, c client, a *pluginsdk.Asset) (string, error) {
	if a.Type == "Subscription" {
		return *a.Name, nil
	}

	if a.Type != "Topic" {
		return "", fmt.Errorf("sample data is only available for Topic and Subscription assets, got %s", a.Type)
	}

	ids, err := c.PullSubscriptionIDs(ctx, *a.Name)
	if err != nil {
		return "", fmt.Errorf("listing subscriptions for topic %s: %w", *a.Name, err)
	}
	if len(ids) == 0 {
		return "", fmt.Errorf("topic %s has no pull subscription to sample from", *a.Name)
	}

	return ids[0], nil
}

func sampleRows(messages []sampleMessage) ([]string, [][]any, error) {
	columns := []string{"message_id", "publish_time", "ordering_key", "attributes", "data"}

	rows := make([][]any, 0, len(messages))
	for _, m := range messages {
		rows = append(rows, []any{
			m.ID,
			m.PublishTime.Format(time.RFC3339),
			m.OrderingKey,
			m.Attributes,
			string(m.Data),
		})
	}

	return columns, rows, nil
}

func (s *Source) connect(ctx context.Context) (client, error) {
	if s.newClient != nil {
		return s.newClient(ctx, s.config)
	}
	return newGoogleClient(ctx, s.config.ProjectID, s.config.EmulatorHost, s.config.CredentialsFile, s.config.CredentialsJSON)
}

// topicURL deep links into the Google Cloud console. An emulator has no
// console, so linking there would be a dead end.
func (s *Source) topicURL(topicID string) string {
	if s.config.EmulatorHost != "" {
		return ""
	}
	return fmt.Sprintf("https://console.cloud.google.com/cloudpubsub/topic/detail/%s?project=%s", topicID, s.config.ProjectID)
}

func (s *Source) subscriptionURL(subscriptionID string) string {
	if s.config.EmulatorHost != "" {
		return ""
	}
	return fmt.Sprintf("https://console.cloud.google.com/cloudpubsub/subscription/detail/%s?project=%s", subscriptionID, s.config.ProjectID)
}

// deliveryType names how a subscription gets its messages. Pub/Sub allows at
// most one export destination, and with none set the subscriber pulls.
func deliveryType(sub subscriptionInfo) string {
	switch {
	case sub.BigQueryTable != "":
		return deliveryBigQuery
	case sub.CloudStorageBucket != "":
		return deliveryCloudStorage
	case sub.PushEndpoint != "":
		return deliveryPush
	default:
		return deliveryPull
	}
}

// bigQueryTableID pulls the table id out of a BigQuery subscription's target,
// which the API returns as "project.dataset.table" or "project:dataset.table".
// The BigQuery plugin names a table by its bare id, so an edge has to match.
func bigQueryTableID(target string) string {
	if target == "" {
		return ""
	}

	parts := strings.FieldsFunc(target, func(r rune) bool {
		return r == '.' || r == ':'
	})
	if len(parts) == 0 {
		return ""
	}

	return parts[len(parts)-1]
}

// kinesisStreamName pulls the stream name out of a Kinesis ARN such as
// "arn:aws:kinesis:us-east-1:111122223333:stream/orders". The Kinesis plugin
// names a stream by that bare name.
func kinesisStreamName(arn string) string {
	if !strings.HasPrefix(arn, "arn:") {
		return ""
	}

	_, name, found := strings.Cut(arn, ":stream/")
	if !found || name == "" {
		return ""
	}

	return name
}

func subscriptionIDs(subs []subscriptionInfo) []string {
	ids := make([]string, 0, len(subs))
	for _, sub := range subs {
		ids = append(ids, sub.ID)
	}
	return ids
}

// assetMRN is the single place a Pub/Sub MRN is built. Topics, subscriptions
// and every lineage edge go through it so the two can never drift into
// addressing the same resource differently. Pub/Sub ids are unique within a
// project, so the bare id is the name.
func assetMRN(assetType, name string) string {
	return mrn.New(assetType, provider, name)
}
