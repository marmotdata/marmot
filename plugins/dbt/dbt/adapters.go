package dbt

// Adapter defines the interface for dbt database adapters.
// Each adapter knows how to map dbt materializations to the correct
// asset types and provider names for its target platform.
type Adapter interface {
	// Name returns the canonical provider name for this adapter (e.g., "Snowflake", "BigQuery")
	Name() string

	// AssetTypeForMaterialization maps a dbt materialization type to the appropriate asset type
	// For most adapters: table -> Table, view -> View, incremental -> Table
	// Some adapters have special types (e.g., ClickHouse has Dictionary, Distributed Table)
	AssetTypeForMaterialization(materialization string) string

	// SupportsSchemas returns true if the adapter uses database.schema.table naming
	// Some platforms like BigQuery use project.dataset.table instead
	SupportsSchemas() bool

	// DefaultMaterialization returns the default materialization if none is specified
	DefaultMaterialization() string
}

// BaseAdapter provides default implementations for common adapter behavior
type BaseAdapter struct {
	ProviderName string
}

func (a *BaseAdapter) Name() string {
	return a.ProviderName
}

func (a *BaseAdapter) AssetTypeForMaterialization(materialization string) string {
	switch materialization {
	case "view":
		return "View"
	case "table", "incremental":
		return "Table"
	case "materialized_view":
		return "Materialized View"
	case "ephemeral":
		return "Ephemeral" // Not actually materialized
	default:
		return "Table"
	}
}

func (a *BaseAdapter) SupportsSchemas() bool {
	return true
}

func (a *BaseAdapter) DefaultMaterialization() string {
	return "view"
}

// AdapterRegistry holds all registered adapters
var adapterRegistry = make(map[string]Adapter)

// RegisterAdapter registers an adapter for a given dbt adapter type
func RegisterAdapter(adapterType string, adapter Adapter) {
	adapterRegistry[adapterType] = adapter
}

// GetAdapter returns the adapter for the given dbt adapter type
// Falls back to a generic DBT adapter if not found
func GetAdapter(adapterType string) Adapter {
	if adapter, ok := adapterRegistry[adapterType]; ok {
		return adapter
	}
	return &GenericAdapter{}
}

// GenericAdapter is the fallback adapter when no specific adapter is registered
type GenericAdapter struct {
	BaseAdapter
}

func (a *GenericAdapter) Name() string {
	return "DBT"
}

func init() {
	// Register all adapters
	registerWarehouseAdapters()
	registerCloudAdapters()
	registerSpecializedAdapters()
}
