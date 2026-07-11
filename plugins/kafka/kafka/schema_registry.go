package kafka

import (
	"fmt"

	"github.com/rs/zerolog/log"
)

type ConsumerGroupDetails struct {
	State        string
	Protocol     string
	ProtocolType string
	Members      []ConsumerGroupMember
}

type ConsumerGroupMember struct {
	ClientID        string
	ClientHost      string
	TopicPartitions map[string][]int
}

func (s *Source) enrichWithSchemaRegistry(topic string, metadata map[string]interface{}) (map[string]string, error) {
	schema := make(map[string]string)

	subjects, err := s.schemaRegistry.GetAllSubjects()
	if err != nil {
		return schema, fmt.Errorf("listing subjects: %w", err)
	}

	// Schema Registry subjects for a topic typically follow the pattern
	// {topic}-value, {topic}-key or {topic}-<custom> (e.g. -headers).
	prefix := topic + "-"
	matchedCount := 0

	for _, subject := range subjects {
		if len(subject) > len(prefix) && subject[:len(prefix)] == prefix {
			schemaMetadata, err := s.schemaRegistry.GetLatestSchemaMetadata(subject)
			if err != nil {
				log.Warn().Err(err).Str("subject", subject).Msg("Failed to get schema for subject")
				continue
			}

			suffix := subject[len(prefix):]

			metadata[suffix+"_schema_id"] = schemaMetadata.ID
			metadata[suffix+"_schema_version"] = schemaMetadata.Version
			metadata[suffix+"_schema_type"] = schemaMetadata.SchemaType

			schema[subject] = schemaMetadata.Schema
			matchedCount++
		}
	}

	log.Debug().
		Str("topic", topic).
		Int("matched_schemas", matchedCount).
		Msg("Enriched topic with Schema Registry data")

	return schema, nil
}
