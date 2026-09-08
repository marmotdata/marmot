package pubsub

import (
	"context"
	"errors"
	"time"

	pluginsdk "github.com/marmotdata/plugin-sdk"
)

// fakeClient stands in for a Pub/Sub project so the mapping from API
// configuration to assets and lineage can be tested without a live project.
// The values below mirror what the emulator returns for the fixtures the e2e
// test seeds, plus the fields the emulator does not implement (schemas,
// ingestion sources, KMS).
type fakeClient struct {
	topics        []topicInfo
	subscriptions map[string][]subscriptionInfo
	schemas       []schemaInfo
	schemasErr    error
	messages      []sampleMessage

	closed bool
}

func (f *fakeClient) Topics(context.Context) ([]topicInfo, error) {
	return f.topics, nil
}

func (f *fakeClient) Subscriptions(_ context.Context, topicID string) ([]subscriptionInfo, error) {
	return f.subscriptions[topicID], nil
}

func (f *fakeClient) Schemas(context.Context) ([]schemaInfo, error) {
	if f.schemasErr != nil {
		return nil, f.schemasErr
	}
	return f.schemas, nil
}

func (f *fakeClient) PullSubscriptionIDs(_ context.Context, topicID string) ([]string, error) {
	var ids []string
	for _, sub := range f.subscriptions[topicID] {
		if deliveryType(sub) == deliveryPull {
			ids = append(ids, sub.ID)
		}
	}
	return ids, nil
}

func (f *fakeClient) ReceiveMessages(_ context.Context, _ string, max int, _ time.Duration) ([]sampleMessage, error) {
	if len(f.messages) > max {
		return f.messages[:max], nil
	}
	return f.messages, nil
}

func (f *fakeClient) Close() error {
	f.closed = true
	return nil
}

const ordersAvroSchema = `{
  "type": "record",
  "name": "Order",
  "fields": [
    {"name": "order_id", "type": "string", "doc": "Unique order identifier"},
    {"name": "amount", "type": ["null", "double"]},
    {"name": "items", "type": {"type": "array", "items": "string"}}
  ]
}`

// newFakeProject builds a project shaped like the one the e2e test seeds:
// an orders topic with four subscriptions covering every delivery type, plus
// a dead letter topic.
func newFakeProject() *fakeClient {
	return &fakeClient{
		topics: []topicInfo{
			{
				ID:                        "orders",
				FullName:                  "projects/test-project/topics/orders",
				Labels:                    map[string]string{"team": "commerce"},
				AllowedPersistenceRegions: []string{"europe-west1"},
				KMSKeyName:                "projects/test-project/locations/eu/keyRings/r/cryptoKeys/k",
				RetentionDuration:         24 * time.Hour,
				State:                     "ACTIVE",
				SchemaName:                "projects/test-project/schemas/order",
				SchemaEncoding:            "JSON",
			},
			{
				ID:       "orders-dlq",
				FullName: "projects/test-project/topics/orders-dlq",
				State:    "ACTIVE",
			},
		},
		subscriptions: map[string][]subscriptionInfo{
			"orders": {
				{
					ID:                  "orders-sub",
					FullName:            "projects/test-project/subscriptions/orders-sub",
					TopicID:             "orders",
					AckDeadline:         30 * time.Second,
					RetentionDuration:   7 * 24 * time.Hour,
					Filter:              `attributes.region = "eu"`,
					Labels:              map[string]string{"team": "commerce"},
					DeadLetterTopicID:   "orders-dlq",
					MaxDeliveryAttempts: 5,
					State:               "ACTIVE",
				},
				{
					ID:           "orders-push",
					FullName:     "projects/test-project/subscriptions/orders-push",
					TopicID:      "orders",
					PushEndpoint: "https://example.com/hooks/orders",
					AckDeadline:  10 * time.Second,
					State:        "ACTIVE",
				},
				{
					ID:            "orders-bq",
					FullName:      "projects/test-project/subscriptions/orders-bq",
					TopicID:       "orders",
					BigQueryTable: "test-project.analytics.orders_raw",
					BigQueryState: "ACTIVE",
					AckDeadline:   10 * time.Second,
					State:         "ACTIVE",
				},
				{
					ID:                 "orders-gcs",
					FullName:           "projects/test-project/subscriptions/orders-gcs",
					TopicID:            "orders",
					CloudStorageBucket: "order-archive",
					CloudStorageState:  "ACTIVE",
					AckDeadline:        10 * time.Second,
					State:              "ACTIVE",
				},
			},
		},
		schemas: []schemaInfo{{
			ID:         "order",
			FullName:   "projects/test-project/schemas/order",
			Type:       "AVRO",
			Definition: ordersAvroSchema,
			RevisionID: "a1b2c3d4",
		}},
	}
}

// newTestSource returns a Source wired to a fake project, with the same
// defaults Validate would apply.
func newTestSource(fake *fakeClient) *Source {
	return &Source{
		config: &Config{
			ProjectID:               "test-project",
			IncludeSubscriptions:    true,
			IncludeSchemas:          true,
			IncludeDeadLetterTopics: true,
		},
		newClient: func(context.Context, *Config) (client, error) { return fake, nil },
	}
}

// assetsByName indexes a discovery result so a test can assert on one asset.
func assetsByName(assets []pluginsdk.Asset) map[string]pluginsdk.Asset {
	byName := make(map[string]pluginsdk.Asset, len(assets))
	for _, a := range assets {
		byName[*a.Name] = a
	}
	return byName
}

// hasEdge reports whether the result contains exactly this edge.
func hasEdge(edges []pluginsdk.LineageEdge, source, target, edgeType string) bool {
	for _, e := range edges {
		if e.Source == source && e.Target == target && e.Type == edgeType {
			return true
		}
	}
	return false
}

// errNoSchemaService is what the emulator answers when asked for schemas.
var errNoSchemaService = errors.New("rpc error: code = Unimplemented desc = unknown service google.pubsub.v1.SchemaService")
