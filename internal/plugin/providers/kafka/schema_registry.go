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

	// Get all subjects from Schema Registry
	subjects, err := s.schemaRegistry.GetAllSubjects()
	if err != nil {
		return schema, fmt.Errorf("listing subjects: %w", err)
	}

	// Filter for subjects that match this topic
	// Typically: {topic}-value, {topic}-key, or other custom patterns like {topic}-headers
	prefix := topic + "-"
	matchedCount := 0

	for _, subject := range subjects {
		// Check if subject starts with topic name followed by hyphen
		if len(subject) > len(prefix) && subject[:len(prefix)] == prefix {
			// Get the schema for this subject
			schemaMetadata, err := s.schemaRegistry.GetLatestSchemaMetadata(subject)
			if err != nil {
				log.Warn().Err(err).Str("subject", subject).Msg("Failed to get schema for subject")
				continue
			}

			// Extract the suffix (e.g., "value", "key", "headers")
			suffix := subject[len(prefix):]

			// Add schema metadata to the main metadata
			metadata[suffix+"_schema_id"] = schemaMetadata.ID
			metadata[suffix+"_schema_version"] = schemaMetadata.Version
			metadata[suffix+"_schema_type"] = schemaMetadata.SchemaType

			// Store the entire schema in the schema map using the full subject name as key
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
