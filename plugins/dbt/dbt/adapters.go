package dbt

import "strings"

// Adapter defines the interface for dbt database adapters.
// Each adapter knows how to map dbt materializations to the correct
// asset types and provider names for its target platform.
type Adapter interface {
	// Name returns the canonical provider name for this adapter (e.g., "Snowflake", "BigQuery")
	// This must be the word the technology's own Marmot plugin uses, because
	// it becomes the service component of the MRN.
	Name() string

	// AssetTypeForMaterialization maps a dbt materialization type to the appropriate asset type
	// For most adapters: table -> Table, view -> View, incremental -> Table
	// Some adapters have special types (e.g., ClickHouse has Dictionary)
	AssetTypeForMaterialization(materialization string) string

	// MRNName builds the name component of the MRN for a table dbt
	// materializes. dbt always knows database, schema and table, but the
	// plugin that owns the table decides how many of those levels are part
	// of its identity: a Postgres table is schema.table, a BigQuery table
	// is dataset.table. Returning anything else files the same physical
	// table a second time instead of merging with the plugin's asset.
	MRNName(database, schema, table string) string

	// DefaultMaterialization returns the default materialization if none is specified
	DefaultMaterialization() string
}

// mrnService is the service component of an MRN for a provider. mrn.New
// sanitizes the name it is given but not the service, so a provider with
// a space in it would put that space into the MRN and into every URL
// built from it. Spaces are dropped rather than hyphenated, matching the
// helper the Trino and OpenMetadata plugins already use.
func mrnService(provider string) string {
	return strings.ReplaceAll(provider, " ", "")
}

// mrnType is the type component of an MRN. mrn.New does not sanitize it
// either, and dbt is the only plugin that mints compound asset types such
// as "Dynamic Table", so they are slugged here rather than left to put a
// space in the MRN.
func mrnType(assetType string) string {
	return strings.ReplaceAll(assetType, " ", "")
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
		// Marmot's own SQL plugins do not distinguish a materialized view
		// from a view (plugins/postgresql, plugins/clickhouse both collapse
		// it), so neither does dbt. The plugins are authoritative.
		return "View"
	case "ephemeral":
		return "Ephemeral" // Not actually materialized
	default:
		return "Table"
	}
}

// MRNName defaults to database.schema.table, which is right for the
// warehouses whose plugin (or projection) keeps all three levels:
// Snowflake, Redshift, SQL Server, Databricks, Synapse, Fabric.
func (a *BaseAdapter) MRNName(database, schema, table string) string {
	return joinNonEmpty(database, schema, table)
}

// joinNonEmpty joins the levels dbt actually filled in. A dbt project
// against an engine with no database concept leaves that field empty, and
// a leading dot would not match any plugin's MRN.
func joinNonEmpty(parts ...string) string {
	kept := parts[:0]
	for _, p := range parts {
		if p != "" {
			kept = append(kept, p)
		}
	}
	return strings.Join(kept, ".")
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
