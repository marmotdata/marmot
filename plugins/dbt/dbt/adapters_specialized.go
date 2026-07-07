package dbt

// ClickHouseAdapter handles ClickHouse OLAP database
type ClickHouseAdapter struct {
	BaseAdapter
}

func (a *ClickHouseAdapter) Name() string {
	return "ClickHouse"
}

func (a *ClickHouseAdapter) AssetTypeForMaterialization(materialization string) string {
	switch materialization {
	case "view":
		return "View"
	case "table", "incremental":
		return "Table"
	case "materialized_view":
		return "Materialized View"
	case "dictionary":
		return "Dictionary"
	case "distributed", "distributed_incremental":
		return "Distributed Table"
	case "ephemeral":
		return "Ephemeral"
	default:
		return "Table"
	}
}

// MaterializeAdapter handles Materialize streaming database
type MaterializeAdapter struct {
	BaseAdapter
}

func (a *MaterializeAdapter) Name() string {
	return "Materialize"
}

func (a *MaterializeAdapter) AssetTypeForMaterialization(materialization string) string {
	switch materialization {
	case "view":
		return "View"
	case "table":
		return "Table"
	case "materializedview":
		return "Materialized View"
	case "source":
		return "Source"
	case "sink":
		return "Sink"
	case "ephemeral":
		return "Ephemeral"
	default:
		return "Materialized View"
	}
}

func (a *MaterializeAdapter) DefaultMaterialization() string {
	return "materializedview"
}

// TrinoAdapter handles Trino/Starburst distributed SQL query engine
type TrinoAdapter struct {
	BaseAdapter
}

func (a *TrinoAdapter) Name() string {
	return "Trino"
}

// TeradataAdapter handles Teradata data warehouse
type TeradataAdapter struct {
	BaseAdapter
}

func (a *TeradataAdapter) Name() string {
	return "Teradata"
}

func (a *TeradataAdapter) AssetTypeForMaterialization(materialization string) string {
	switch materialization {
	case "view":
		return "View"
	case "table", "incremental":
		return "Table"
	case "ephemeral":
		return "Ephemeral"
	default:
		return "View"
	}
}

// OracleAdapter handles Oracle Database / Autonomous Database
type OracleAdapter struct {
	BaseAdapter
}

func (a *OracleAdapter) Name() string {
	return "Oracle"
}

func (a *OracleAdapter) AssetTypeForMaterialization(materialization string) string {
	switch materialization {
	case "view":
		return "View"
	case "table", "incremental":
		return "Table"
	case "materialized_view":
		return "Materialized View"
	case "ephemeral":
		return "Ephemeral"
	default:
		return "Table"
	}
}

// NetezzaAdapter handles IBM Netezza
type NetezzaAdapter struct {
	BaseAdapter
}

func (a *NetezzaAdapter) Name() string {
	return "Netezza"
}

// SalesforceAdapter handles Salesforce Data Cloud
type SalesforceAdapter struct {
	BaseAdapter
}

func (a *SalesforceAdapter) Name() string {
	return "Salesforce"
}

func (a *SalesforceAdapter) AssetTypeForMaterialization(materialization string) string {
	switch materialization {
	case "view":
		return "View"
	case "table", "incremental":
		return "Data Model Object"
	case "ephemeral":
		return "Ephemeral"
	default:
		return "Data Model Object"
	}
}

func registerSpecializedAdapters() {
	RegisterAdapter("clickhouse", &ClickHouseAdapter{})
	RegisterAdapter("materialize", &MaterializeAdapter{})
	RegisterAdapter("trino", &TrinoAdapter{})
	RegisterAdapter("starburst", &TrinoAdapter{})
	RegisterAdapter("teradata", &TeradataAdapter{})
	RegisterAdapter("oracle", &OracleAdapter{})
	RegisterAdapter("ibm_netezza", &NetezzaAdapter{})
	RegisterAdapter("salesforce", &SalesforceAdapter{})
}
