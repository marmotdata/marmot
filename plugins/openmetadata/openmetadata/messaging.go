package openmetadata

import (
	"context"
	"fmt"
	"strings"

	"github.com/rs/zerolog/log"
)

const topicFields = "owners,tags,domains,dataProducts,messageSchema"

// discoverTopics catalogues messaging topics. A topic's fully qualified
// name is service.topic, so the name below the service is the topic name
// Marmot's Kafka plugin already uses.
func (c *collector) discoverTopics(ctx context.Context, client *client) error {
	topics, err := listAll[topic](ctx, client, "/v1/topics", topicFields, c.config.PageSize, c.config.IncludeDeleted)
	if err != nil {
		return fmt.Errorf("listing topics: %w", err)
	}

	discovered := 0
	for _, t := range topics {
		if !c.wanted(t.entityBase) {
			continue
		}

		p := projectionFor(t.ServiceType)
		topicName := strings.Join(fqnBelowService(t.FullyQualifiedName), ".")
		if topicName == "" {
			continue
		}

		metadata := map[string]interface{}{}
		putIf(metadata, "partitions", t.Partitions)
		putIf(metadata, "replication_factor", t.ReplicationFactor)
		putIf(metadata, "retention_size", int64(t.RetentionSize))
		putIf(metadata, "retention_ms", int64(t.RetentionTime))
		putIf(metadata, "max_message_size", t.MaximumMessageSize)
		putIf(metadata, "cleanup_policies", t.CleanupPolicies)
		if t.MessageSchema != nil {
			putIf(metadata, "schema_type", t.MessageSchema.SchemaType)
		}

		asset := c.newAsset(t.entityBase, "topic", "Topic", p, c.mrnName(topicName, t.FullyQualifiedName), metadata)

		if t.MessageSchema != nil {
			if c.config.IncludeColumns {
				setColumns(&asset, t.MessageSchema.SchemaFields)
			}
			if schemaText := strings.TrimSpace(t.MessageSchema.SchemaText); schemaText != "" {
				asset.Schema["schema"] = schemaText
			}
		}

		c.add(t.ID, asset)
		discovered++
	}

	log.Debug().Int("count", discovered).Msg("Discovered topics")
	return nil
}
