package pubsub

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	gpubsub "cloud.google.com/go/pubsub"
	"github.com/rs/zerolog/log"
	"google.golang.org/api/iterator"
	"google.golang.org/api/option"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// maxListed bounds every paged listing so a project with a runaway number of
// resources cannot make discovery run forever.
const maxListed = 100000

// topicInfo is the part of a Pub/Sub topic's configuration the plugin
// records. Discovery works on these plain structs rather than on the Google
// client types so the mapping to assets can be tested without a live project.
type topicInfo struct {
	ID       string
	FullName string

	Labels                    map[string]string
	KMSKeyName                string
	AllowedPersistenceRegions []string
	RetentionDuration         time.Duration
	State                     string

	SchemaName            string
	SchemaEncoding        string
	SchemaFirstRevisionID string
	SchemaLastRevisionID  string

	// IngestionSource names the external system Pub/Sub pulls messages from,
	// empty when the topic is only written to by publishers.
	IngestionSource           string
	IngestionKinesisStreamARN string
	IngestionBucket           string
}

// subscriptionInfo is the part of a Pub/Sub subscription's configuration the
// plugin records.
type subscriptionInfo struct {
	ID       string
	FullName string
	TopicID  string

	PushEndpoint string

	BigQueryTable          string
	BigQueryUseTopicSchema bool
	BigQueryState          string

	CloudStorageBucket         string
	CloudStorageFilenamePrefix string
	CloudStorageFilenameSuffix string
	CloudStorageState          string

	AckDeadline           time.Duration
	RetainAckedMessages   bool
	RetentionDuration     time.Duration
	ExpirationTTL         time.Duration
	Labels                map[string]string
	EnableMessageOrdering bool
	DeadLetterTopicID     string
	MaxDeliveryAttempts   int
	Filter                string
	Detached              bool
	ExactlyOnceDelivery   bool
	State                 string
}

// schemaInfo is a Pub/Sub schema definition, keyed by its full resource name.
type schemaInfo struct {
	ID         string
	FullName   string
	Type       string
	Definition string
	RevisionID string
}

// sampleMessage is one message read back for an asset preview.
type sampleMessage struct {
	ID          string
	Data        []byte
	Attributes  map[string]string
	PublishTime time.Time
	OrderingKey string
}

// client is the slice of the Pub/Sub API discovery needs. Tests substitute a
// fake so the asset and lineage mapping can be exercised without a project.
type client interface {
	Topics(ctx context.Context) ([]topicInfo, error)
	Subscriptions(ctx context.Context, topicID string) ([]subscriptionInfo, error)
	Schemas(ctx context.Context) ([]schemaInfo, error)
	PullSubscriptionIDs(ctx context.Context, topicID string) ([]string, error)
	ReceiveMessages(ctx context.Context, subscriptionID string, max int, wait time.Duration) ([]sampleMessage, error)
	Close() error
}

// googleClient talks to the real Pub/Sub API.
type googleClient struct {
	pub     *gpubsub.Client
	schemas *gpubsub.SchemaClient
}

// newGoogleClient connects to Pub/Sub, or to an emulator when emulatorHost is
// set. An emulator speaks plaintext gRPC and has no credentials, so both are
// turned off explicitly rather than relying on the PUBSUB_EMULATOR_HOST
// environment variable, which the ingest process may not have.
func newGoogleClient(ctx context.Context, projectID, emulatorHost, credentialsFile, credentialsJSON string) (*googleClient, error) {
	var opts []option.ClientOption

	switch {
	case emulatorHost != "":
		opts = append(opts,
			option.WithEndpoint(emulatorHost),
			option.WithoutAuthentication(),
			option.WithGRPCDialOption(grpc.WithTransportCredentials(insecure.NewCredentials())),
		)
	case credentialsJSON != "":
		opts = append(opts, option.WithCredentialsJSON([]byte(credentialsJSON)))
	case credentialsFile != "":
		opts = append(opts, option.WithCredentialsFile(credentialsFile))
	}

	pub, err := gpubsub.NewClient(ctx, projectID, opts...)
	if err != nil {
		return nil, fmt.Errorf("creating Pub/Sub client: %w", err)
	}

	c := &googleClient{pub: pub}

	// Schemas live behind a second client. It is optional: an emulator does
	// not implement the schema service at all, so a failure here must not
	// stop topic discovery.
	schemas, err := gpubsub.NewSchemaClient(ctx, projectID, opts...)
	if err == nil {
		c.schemas = schemas
	}

	return c, nil
}

func (c *googleClient) Close() error {
	if c.schemas != nil {
		c.schemas.Close()
	}
	if c.pub != nil {
		return c.pub.Close()
	}
	return nil
}

func (c *googleClient) Topics(ctx context.Context) ([]topicInfo, error) {
	it := c.pub.Topics(ctx)

	var topics []topicInfo
	for {
		if len(topics) >= maxListed {
			log.Warn().Int("limit", maxListed).Msg("Stopped listing topics at the limit")
			break
		}

		cfg, err := it.NextConfig()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return topics, fmt.Errorf("listing topics: %w", err)
		}
		topics = append(topics, topicInfoFromConfig(cfg))
	}

	return topics, nil
}

func (c *googleClient) Subscriptions(ctx context.Context, topicID string) ([]subscriptionInfo, error) {
	it := c.pub.Topic(topicID).Subscriptions(ctx)

	var subs []subscriptionInfo
	for {
		if len(subs) >= maxListed {
			log.Warn().Str("topic", topicID).Int("limit", maxListed).Msg("Stopped listing subscriptions at the limit")
			break
		}

		sub, err := it.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return subs, fmt.Errorf("listing subscriptions for topic %s: %w", topicID, err)
		}

		// ListTopicSubscriptions returns names only, so each subscription's
		// settings need their own call.
		cfg, err := sub.Config(ctx)
		if err != nil {
			return subs, fmt.Errorf("reading subscription %s: %w", sub.ID(), err)
		}
		subs = append(subs, subscriptionInfoFromConfig(sub.ID(), sub.String(), &cfg))
	}

	return subs, nil
}

func (c *googleClient) Schemas(ctx context.Context) ([]schemaInfo, error) {
	if c.schemas == nil {
		return nil, fmt.Errorf("schema client unavailable")
	}

	it := c.schemas.Schemas(ctx, gpubsub.SchemaViewFull)

	var schemas []schemaInfo
	for {
		if len(schemas) >= maxListed {
			log.Warn().Int("limit", maxListed).Msg("Stopped listing schemas at the limit")
			break
		}

		cfg, err := it.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return schemas, fmt.Errorf("listing schemas: %w", err)
		}
		schemas = append(schemas, schemaInfo{
			ID:         lastSegment(cfg.Name),
			FullName:   cfg.Name,
			Type:       schemaTypeName(cfg.Type),
			Definition: cfg.Definition,
			RevisionID: cfg.RevisionID,
		})
	}

	return schemas, nil
}

func (c *googleClient) PullSubscriptionIDs(ctx context.Context, topicID string) ([]string, error) {
	subs, err := c.Subscriptions(ctx, topicID)
	if err != nil {
		return nil, err
	}

	var ids []string
	for _, sub := range subs {
		if deliveryType(sub) == deliveryPull {
			ids = append(ids, sub.ID)
		}
	}
	return ids, nil
}

// ReceiveMessages reads up to max messages from a subscription, or fewer if
// wait elapses first. Every message is nacked so the sample does not consume
// the subscription's backlog; Pub/Sub redelivers them to the real consumer.
func (c *googleClient) ReceiveMessages(ctx context.Context, subscriptionID string, max int, wait time.Duration) ([]sampleMessage, error) {
	sub := c.pub.Subscription(subscriptionID)
	sub.ReceiveSettings.NumGoroutines = 1
	sub.ReceiveSettings.MaxOutstandingMessages = max

	recvCtx, cancel := context.WithTimeout(ctx, wait)
	defer cancel()

	var (
		messages []sampleMessage
		mu       sync.Mutex
	)

	err := sub.Receive(recvCtx, func(_ context.Context, m *gpubsub.Message) {
		m.Nack()

		mu.Lock()
		defer mu.Unlock()
		if len(messages) >= max {
			return
		}
		messages = append(messages, sampleMessage{
			ID:          m.ID,
			Data:        m.Data,
			Attributes:  m.Attributes,
			PublishTime: m.PublishTime,
			OrderingKey: m.OrderingKey,
		})
		if len(messages) >= max {
			cancel()
		}
	})
	// A cancelled context is how the read is stopped, both on the timeout and
	// once max messages are in hand, so it is not an error.
	if err != nil && recvCtx.Err() == nil {
		return nil, fmt.Errorf("receiving from subscription %s: %w", subscriptionID, err)
	}

	mu.Lock()
	defer mu.Unlock()
	return messages, nil
}

func topicInfoFromConfig(cfg *gpubsub.TopicConfig) topicInfo {
	info := topicInfo{
		ID:                        cfg.ID(),
		FullName:                  cfg.String(),
		Labels:                    cfg.Labels,
		KMSKeyName:                cfg.KMSKeyName,
		AllowedPersistenceRegions: cfg.MessageStoragePolicy.AllowedPersistenceRegions,
		State:                     topicStateName(cfg.State),
	}

	if cfg.RetentionDuration != nil {
		if d, ok := cfg.RetentionDuration.(time.Duration); ok {
			info.RetentionDuration = d
		}
	}

	if cfg.SchemaSettings != nil {
		info.SchemaName = cfg.SchemaSettings.Schema
		info.SchemaEncoding = schemaEncodingName(cfg.SchemaSettings.Encoding)
		info.SchemaFirstRevisionID = cfg.SchemaSettings.FirstRevisionID
		info.SchemaLastRevisionID = cfg.SchemaSettings.LastRevisionID
	}

	if cfg.IngestionDataSourceSettings != nil {
		switch src := cfg.IngestionDataSourceSettings.Source.(type) {
		case *gpubsub.IngestionDataSourceAWSKinesis:
			info.IngestionSource = "aws_kinesis"
			info.IngestionKinesisStreamARN = src.StreamARN
		case *gpubsub.IngestionDataSourceCloudStorage:
			info.IngestionSource = "cloud_storage"
			info.IngestionBucket = src.Bucket
		case *gpubsub.IngestionDataSourceAzureEventHubs:
			info.IngestionSource = "azure_event_hubs"
		case *gpubsub.IngestionDataSourceAmazonMSK:
			info.IngestionSource = "amazon_msk"
		case *gpubsub.IngestionDataSourceConfluentCloud:
			info.IngestionSource = "confluent_cloud"
		}
	}

	return info
}

func subscriptionInfoFromConfig(id, fullName string, cfg *gpubsub.SubscriptionConfig) subscriptionInfo {
	info := subscriptionInfo{
		ID:                     id,
		FullName:               fullName,
		PushEndpoint:           cfg.PushConfig.Endpoint,
		BigQueryTable:          cfg.BigQueryConfig.Table,
		BigQueryUseTopicSchema: cfg.BigQueryConfig.UseTopicSchema,
		AckDeadline:            cfg.AckDeadline,
		RetainAckedMessages:    cfg.RetainAckedMessages,
		RetentionDuration:      cfg.RetentionDuration,
		Labels:                 cfg.Labels,
		EnableMessageOrdering:  cfg.EnableMessageOrdering,
		Filter:                 cfg.Filter,
		Detached:               cfg.Detached,
		ExactlyOnceDelivery:    cfg.EnableExactlyOnceDelivery,
		State:                  subscriptionStateName(cfg.State),
	}

	if cfg.Topic != nil {
		info.TopicID = cfg.Topic.ID()
	}
	if cfg.BigQueryConfig.Table != "" {
		info.BigQueryState = bigQueryStateName(cfg.BigQueryConfig.State)
	}
	if cfg.CloudStorageConfig.Bucket != "" {
		info.CloudStorageBucket = cfg.CloudStorageConfig.Bucket
		info.CloudStorageFilenamePrefix = cfg.CloudStorageConfig.FilenamePrefix
		info.CloudStorageFilenameSuffix = cfg.CloudStorageConfig.FilenameSuffix
		info.CloudStorageState = cloudStorageStateName(cfg.CloudStorageConfig.State)
	}
	if cfg.ExpirationPolicy != nil {
		if d, ok := cfg.ExpirationPolicy.(time.Duration); ok {
			info.ExpirationTTL = d
		}
	}
	if cfg.DeadLetterPolicy != nil {
		info.DeadLetterTopicID = lastSegment(cfg.DeadLetterPolicy.DeadLetterTopic)
		info.MaxDeliveryAttempts = cfg.DeadLetterPolicy.MaxDeliveryAttempts
	}

	return info
}

func topicStateName(state gpubsub.TopicState) string {
	switch state {
	case gpubsub.TopicStateActive:
		return "ACTIVE"
	case gpubsub.TopicStateIngestionResourceError:
		return "INGESTION_RESOURCE_ERROR"
	default:
		return ""
	}
}

func subscriptionStateName(state gpubsub.SubscriptionState) string {
	switch state {
	case gpubsub.SubscriptionStateActive:
		return "ACTIVE"
	case gpubsub.SubscriptionStateResourceError:
		return "RESOURCE_ERROR"
	default:
		return ""
	}
}

func bigQueryStateName(state gpubsub.BigQueryConfigState) string {
	if state == gpubsub.BigQueryConfigActive {
		return "ACTIVE"
	}
	return ""
}

func cloudStorageStateName(state gpubsub.CloudStorageConfigState) string {
	if state == gpubsub.CloudStorageConfigActive {
		return "ACTIVE"
	}
	return ""
}

func schemaTypeName(t gpubsub.SchemaType) string {
	switch t {
	case gpubsub.SchemaAvro:
		return "AVRO"
	case gpubsub.SchemaProtocolBuffer:
		return "PROTOCOL_BUFFER"
	default:
		return ""
	}
}

func schemaEncodingName(e gpubsub.SchemaEncoding) string {
	switch e {
	case gpubsub.EncodingJSON:
		return "JSON"
	case gpubsub.EncodingBinary:
		return "BINARY"
	default:
		return ""
	}
}

// lastSegment returns the id at the end of a resource name such as
// "projects/p/topics/orders".
func lastSegment(name string) string {
	if i := strings.LastIndex(name, "/"); i >= 0 {
		return name[i+1:]
	}
	return name
}
