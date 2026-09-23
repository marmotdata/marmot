package dataproduct

import (
	"github.com/marmotdata/marmot/internal/core/metamodel"
)

// validateMetamodel checks metadata against the governed data_product fields. Create/Update API, never partial ingestion — so every write is fully governed.
func validateMetamodel(registry *metamodel.Registry, metadata map[string]interface{}) error {
	if registry == nil || !registry.Enabled() {
		return nil
	}
	values := make(map[string]any)
	for _, f := range registry.Fields("data_product") {
		if value, present := metamodel.ValueAt(metadata, f.Storage); present {
			values[f.ID] = value
		}
	}
	return registry.Validate(values, "data_product", true)
}
