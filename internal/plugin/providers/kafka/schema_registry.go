package kafka

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

func (s *Source) enrichWithSchemaRegistry(topic string, metadata map[string]interface{}) error {
	valueSubject := topic + "-value"
	valueMetadata, err := s.schemaRegistry.GetLatestSchemaMetadata(valueSubject)
	if err == nil {
		metadata["value_schema_id"] = valueMetadata.ID
		metadata["value_schema_version"] = valueMetadata.Version
		metadata["value_schema_type"] = valueMetadata.SchemaType
		metadata["value_schema"] = valueMetadata.Schema
	}

	keySubject := topic + "-key"
	keyMetadata, err := s.schemaRegistry.GetLatestSchemaMetadata(keySubject)
	if err == nil {
		metadata["key_schema_id"] = keyMetadata.ID
		metadata["key_schema_version"] = keyMetadata.Version
		metadata["key_schema_type"] = keyMetadata.SchemaType
		metadata["key_schema"] = keyMetadata.Schema
	}

	return nil
}
