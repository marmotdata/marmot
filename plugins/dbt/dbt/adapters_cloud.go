package dbt

// DatabricksAdapter handles Databricks
type DatabricksAdapter struct {
	BaseAdapter
}

func (a *DatabricksAdapter) Name() string {
	return "Databricks"
}

func (a *DatabricksAdapter) AssetTypeForMaterialization(materialization string) string {
	switch materialization {
	case "view":
		return "View"
	case "table", "incremental":
		return "Table"
	case "materialized_view":
		return "Materialized View"
	case "streaming_table":
		return "Streaming Table"
	case "ephemeral":
		return "Ephemeral"
	default:
		return "Table"
	}
}

// SparkAdapter handles Apache Spark
type SparkAdapter struct {
	BaseAdapter
}

func (a *SparkAdapter) Name() string {
	return "Spark"
}

// AthenaAdapter handles AWS Athena
type AthenaAdapter struct {
	BaseAdapter
}

func (a *AthenaAdapter) Name() string {
	return "Athena"
}

func (a *AthenaAdapter) AssetTypeForMaterialization(materialization string) string {
	switch materialization {
	case "view":
		return "View"
	case "table", "incremental":
		return "Table"
	case "table_hive_ha":
		return "Table"
	case "ephemeral":
		return "Ephemeral"
	default:
		return "Table"
	}
}

// GlueAdapter handles AWS Glue
type GlueAdapter struct {
	BaseAdapter
}

func (a *GlueAdapter) Name() string {
	return "AWS Glue"
}

// FabricAdapter handles Microsoft Fabric Data Warehouse
type FabricAdapter struct {
	BaseAdapter
}

func (a *FabricAdapter) Name() string {
	return "Microsoft Fabric"
}

// FabricSparkAdapter handles Microsoft Fabric Lakehouse
type FabricSparkAdapter struct {
	BaseAdapter
}

func (a *FabricSparkAdapter) Name() string {
	return "Microsoft Fabric"
}

func (a *FabricSparkAdapter) AssetTypeForMaterialization(materialization string) string {
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

// DremioAdapter handles Dremio data lakehouse platform
type DremioAdapter struct {
	BaseAdapter
}

func (a *DremioAdapter) Name() string {
	return "Dremio"
}

// LakebaseAdapter handles Databricks Lakebase
type LakebaseAdapter struct {
	BaseAdapter
}

func (a *LakebaseAdapter) Name() string {
	return "Databricks"
}

func registerCloudAdapters() {
	RegisterAdapter("databricks", &DatabricksAdapter{})
	RegisterAdapter("lakebase", &LakebaseAdapter{})
	RegisterAdapter("spark", &SparkAdapter{})
	RegisterAdapter("athena", &AthenaAdapter{})
	RegisterAdapter("glue", &GlueAdapter{})
	RegisterAdapter("fabric", &FabricAdapter{})
	RegisterAdapter("fabricspark", &FabricSparkAdapter{})
	RegisterAdapter("dremio", &DremioAdapter{})
}
