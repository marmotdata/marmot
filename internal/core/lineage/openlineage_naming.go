package lineage

import (
	"net/url"
	"path"
	"slices"
	"strings"

	"github.com/marmotdata/plugin-sdk/mrn"
)

// OpenLineage names a dataset by a namespace, the address of the system holding it, and a name, the path inside that system (https://openlineage.io/docs/spec/naming). Marmot names an asset by type, provider and the path the system's own plugin uses, so a dataset an event mentions and the asset a plugin catalogues are one asset. This file turns the first into the second.

// datasetIdentity is who an OpenLineage dataset is in Marmot.
type datasetIdentity struct {
	// MRN is the asset's MRN, before any alias is followed.
	MRN      string
	Provider string
	Type     string
	// Name is the short name the asset is shown under.
	Name string
	// Legacy lists the MRNs earlier Marmot releases filed the same dataset under, so an existing asset there is adopted rather than duplicated.
	Legacy []string
}

// jobIdentity is who an OpenLineage job is in Marmot.
type jobIdentity struct {
	MRN      string
	Provider string
	Type     string
	Name     string
	Legacy   []string
}

// datasetSystem is what a namespace scheme says about the datasets under it.
type datasetSystem struct {
	provider string
	// assetType is the type a dataset gets when no datasetType facet says otherwise; empty means it depends on the name.
	assetType string
	// layout is how the dataset name splits into the levels of the MRN.
	layout nameLayout
	// noHost leaves the address out of the MRN, for systems whose namespace is not a host.
	noHost bool
}

type nameLayout int

const (
	// dotted names are levels joined with dots, such as database.schema.table.
	dotted nameLayout = iota
	// single names are one level used as they are, such as a Kafka topic or a file path.
	single
	// bucketKey names are an object key inside the bucket or container the namespace names.
	bucketKey
	// glueARN names are table/<database>/<table> under an AWS Glue catalogue ARN.
	glueARN
	// athena names are <catalog>.<database>.<table>, filed under Glue as the Athena plugin does.
	athena
	// pubsub names are topic:<project>:<topic> or subscription:<project>:<subscription>.
	pubsub
)

// datasetSystems maps the namespace schemes in the OpenLineage naming spec to the provider and levels Marmot's plugins use for the same system. Any other scheme keeps the naming Marmot 0.13 used.
var datasetSystems = map[string]datasetSystem{
	"postgres":     {provider: "PostgreSQL", assetType: AssetTypeTable},
	"postgresql":   {provider: "PostgreSQL", assetType: AssetTypeTable},
	"mysql":        {provider: "MySQL", assetType: AssetTypeTable},
	"mariadb":      {provider: "MariaDB", assetType: AssetTypeTable},
	"mssql":        {provider: "SQL Server", assetType: AssetTypeTable},
	"sqlserver":    {provider: "SQL Server", assetType: AssetTypeTable},
	"oracle":       {provider: "Oracle", assetType: AssetTypeTable},
	"db2":          {provider: "Db2", assetType: AssetTypeTable},
	"teradata":     {provider: "Teradata", assetType: AssetTypeTable},
	"cassandra":    {provider: "Cassandra", assetType: AssetTypeTable},
	"hive":         {provider: "Hive", assetType: AssetTypeTable},
	"trino":        {provider: "Trino", assetType: AssetTypeTable},
	"presto":       {provider: "Presto", assetType: AssetTypeTable},
	"clickhouse":   {provider: "ClickHouse", assetType: AssetTypeTable},
	"redshift":     {provider: "Redshift", assetType: AssetTypeTable},
	"snowflake":    {provider: "Snowflake", assetType: AssetTypeTable},
	"unitycatalog": {provider: "Unity Catalog", assetType: AssetTypeTable},
	"bigquery":     {provider: ProviderBigQuery, assetType: AssetTypeTable, noHost: true},
	"awsathena":    {provider: "Glue", assetType: AssetTypeTable, layout: athena, noHost: true},
	"arn:aws:glue": {provider: "Glue", assetType: AssetTypeTable, layout: glueARN, noHost: true},
	"kafka":        {provider: ProviderKafka, assetType: AssetTypeTopic, layout: single},
	"pubsub":       {provider: "GooglePubSub", layout: pubsub, noHost: true},
	"s3":           {provider: ProviderS3, layout: bucketKey, noHost: true},
	"s3a":          {provider: ProviderS3, layout: bucketKey, noHost: true},
	"s3n":          {provider: ProviderS3, layout: bucketKey, noHost: true},
	"gs":           {provider: ProviderGCS, layout: bucketKey, noHost: true},
	"gcs":          {provider: ProviderGCS, layout: bucketKey, noHost: true},
	"abfss":        {provider: "AzureBlob", layout: bucketKey, noHost: true},
	"abfs":         {provider: "AzureBlob", layout: bucketKey, noHost: true},
	"wasbs":        {provider: "AzureBlob", layout: bucketKey, noHost: true},
	"wasb":         {provider: "AzureBlob", layout: bucketKey, noHost: true},
	"hdfs":         {provider: "HDFS", layout: single},
	"dbfs":         {provider: "DBFS", layout: single, noHost: true},
	"file":         {provider: "File", layout: single},
	// dbt-duckdb names its namespace after the database file, duckdb://<path>, which is not a host.
	"duckdb":        {provider: "DuckDB", assetType: AssetTypeTable, noHost: true},
	duckDBNamespace: {provider: "DuckDB", assetType: AssetTypeTable, noHost: true},
}

// duckDBNamespace is a key in datasetSystems for a table in a DuckDB database file, which has no namespace scheme of its own.
const duckDBNamespace = "duckdb+native"

// bareSchemes are namespaces the naming spec writes as a bare word rather than a URL.
var bareSchemes = map[string]bool{"bigquery": true, "pubsub": true, "file": true}

// dataFileExtensions mark an object-store or file dataset as a single file rather than a directory or prefix.
var dataFileExtensions = []string{".parquet", ".csv", ".tsv", ".json", ".jsonl", ".ndjson", ".avro", ".orc", ".txt", ".xlsx", ".xml"}

// datasetTypeFacetTypes maps the datasetType facet onto the asset types plugins use.
var datasetTypeFacetTypes = map[string]string{
	"TABLE":  AssetTypeTable,
	"VIEW":   "View",
	"TOPIC":  AssetTypeTopic,
	"STREAM": "Stream",
	"FILE":   AssetTypeFile,
	"MODEL":  AssetTypeModel,
}

// parseNamespace splits a dataset namespace into its scheme and its authority, the part before any path. A namespace that is not an address, such as a scheduler's name, has no scheme.
func parseNamespace(ns string) (scheme, authority string) {
	ns = strings.TrimSpace(ns)
	lower := strings.ToLower(ns)
	if strings.HasPrefix(lower, "arn:aws:glue:") {
		return "arn:aws:glue", strings.TrimPrefix(ns, ns[:len("arn:aws:glue:")])
	}
	if i := strings.Index(ns, "://"); i > 0 {
		rest := ns[i+3:]
		if j := strings.IndexAny(rest, "/?#"); j >= 0 {
			rest = rest[:j]
		}
		return strings.ToLower(ns[:i]), rest
	}
	if bareSchemes[lower] {
		return lower, ""
	}
	return "", ""
}

// hostOf normalises an authority into the host:port Marmot matches zone bindings on, adding the default port of a database that leaves it out.
func hostOf(scheme, authority string) string {
	if at := strings.LastIndex(authority, "@"); at >= 0 {
		authority = authority[at+1:]
	}
	host := strings.ToLower(authority)
	if host == "" {
		return ""
	}
	if !strings.Contains(strings.TrimPrefix(host, "["), ":") || strings.HasSuffix(host, "]") {
		if port, ok := defaultPorts[scheme]; ok {
			host += ":" + port
		}
	}
	return host
}

func isDataFile(name string) bool {
	ext := strings.ToLower(path.Ext(strings.TrimSuffix(name, "/")))
	return slices.Contains(dataFileExtensions, ext)
}

// identifyDataset works out the asset an OpenLineage dataset is. The namespace and name come first; when the namespace is not an address, as with DuckDB's duck_lineage which files every table under its own namespace, the dataSource and catalog facets say where the data really lives.
func identifyDataset(ds *Dataset, producer string) datasetIdentity {
	ns, name, native := resolveDatasetAddress(ds, producer)
	scheme, authority := parseNamespace(ns)
	if native {
		scheme = duckDBNamespace
	}
	sys, ok := datasetSystems[scheme]

	id := datasetIdentity{Legacy: legacyDatasetMRNs(ds)}
	if !ok {
		// A namespace Marmot has no mapping for keeps the 0.13 naming, so assets filed under it stay where they are.
		id.Provider = legacyInferDatasetProvider(ds)
		id.Type = legacyInferDatasetType(ds)
		id.MRN = openLineageDatasetMRN(id.Type, id.Provider, ds.Namespace, ds.Name)
		id.Name = ds.Name
		id.Legacy = slices.DeleteFunc(id.Legacy, func(m string) bool { return m == id.MRN })
		return id
	}

	id.Provider = sys.provider
	id.Type = sys.assetType
	var levels []string
	switch sys.layout {
	case dotted:
		levels = strings.Split(name, ".")
	case single:
		levels = []string{name}
		if id.Type == "" {
			id.Type = AssetTypeDataset
			if isDataFile(name) {
				id.Type = AssetTypeFile
			}
		}
	case bucketKey:
		bucket := authority
		if strings.HasPrefix(scheme, "ab") || strings.HasPrefix(scheme, "wasb") {
			// Azure names the container before the @ and the storage account after it.
			bucket, _, _ = strings.Cut(authority, "@")
		}
		key := strings.Trim(name, "/")
		levels = []string{bucket}
		id.Type = AssetTypeBucket
		if key != "" {
			levels = append(levels, key)
			if isDataFile(key) {
				id.Type = AssetTypeFile
			}
		}
	case glueARN:
		parts := strings.Split(strings.TrimPrefix(name, "table/"), "/")
		levels = parts
	case athena:
		levels = strings.Split(name, ".")
		if len(levels) == 3 && strings.EqualFold(levels[0], "awsdatacatalog") {
			levels = levels[1:]
		}
	case pubsub:
		kind, rest, _ := strings.Cut(name, ":")
		_, resource, found := strings.Cut(rest, ":")
		if !found {
			resource = rest
		}
		levels = []string{resource}
		id.Type = AssetTypeTopic
		if kind == "subscription" {
			id.Type = "Subscription"
		}
	}
	if t := datasetTypeFromFacet(ds); t != "" && (sys.assetType == AssetTypeTable || sys.assetType == "") && sys.layout != bucketKey {
		id.Type = t
	}

	m := mrn.Asset(id.Type, id.Provider, levels...)
	if !sys.noHost {
		if host := hostOf(scheme, authority); host != "" {
			m = m.WithHost(host)
		}
	}
	id.MRN = m.String()
	id.Name = levels[len(levels)-1]
	if sys.layout == single || sys.layout == bucketKey {
		id.Name = path.Base(strings.TrimSuffix(id.Name, "/"))
	}
	id.Legacy = slices.DeleteFunc(id.Legacy, func(m string) bool { return m == id.MRN })
	return id
}

func datasetTypeFromFacet(ds *Dataset) string {
	f := facetMap(ds.Facets, "datasetType")
	t, _ := f["datasetType"].(string)
	return datasetTypeFacetTypes[strings.ToUpper(t)]
}

// resolveDatasetAddress returns the namespace and name a dataset should be filed under, and whether it is a table in a DuckDB database file. A namespace that is an address is used as it is. Otherwise the dataSource facet's URI, then the catalog facet's metadataUri, is used when it is the address of a database: duck_lineage files a table in an attached Postgres database as namespace duckdb and name <alias>.<schema>.<table>, where the alias is the catalog facet's name, so the alias is swapped for the real database name.
func resolveDatasetAddress(ds *Dataset, producer string) (ns, name string, nativeDuckDB bool) {
	ns, name = ds.Namespace, ds.Name
	if scheme, _ := parseNamespace(ns); scheme != "" {
		return ns, name, false
	}
	catalog := facetMap(ds.Facets, "catalog")
	framework, _ := catalog["framework"].(string)
	catalogType, _ := catalog["type"].(string)
	alias, _ := catalog["name"].(string)
	duckDB := strings.EqualFold(framework, "duckdb") || strings.Contains(strings.ToLower(producer), "duck_lineage")

	uri := ""
	if src := facetMap(ds.Facets, "dataSource"); src != nil {
		uri, _ = src["uri"].(string)
	}
	if uri == "" {
		uri, _ = catalog["metadataUri"].(string)
	}

	if duckDB && (strings.EqualFold(catalogType, "native") || catalogType == "" && uri == "" || strings.HasPrefix(strings.ToLower(uri), "file:")) {
		return ns, name, true
	}

	u, err := url.Parse(uri)
	if uri == "" || err != nil || u.Host == "" {
		return ns, name, false
	}
	scheme := strings.ToLower(u.Scheme)
	if sys, ok := datasetSystems[scheme]; !ok || sys.layout != dotted || sys.noHost {
		return ns, name, false
	}
	ns = scheme + "://" + u.Host
	if duckDB && alias != "" {
		parts := strings.Split(name, ".")
		if len(parts) > 1 && parts[0] == alias {
			database := strings.Trim(u.Path, "/")
			if i := strings.Index(database, "/"); i >= 0 {
				database = database[:i]
			}
			switch {
			case scheme == "mysql" || scheme == "mariadb":
				// DuckDB names a MySQL table <alias>.<database>.<table>, and MySQL names it <database>.<table>.
				parts = parts[1:]
			case database != "":
				parts[0] = database
			}
			name = strings.Join(parts, ".")
		}
	}
	return ns, name, false
}

// cleanURI returns a data source URI fit to store: scheme, host, port and path, never credentials or query parameters, and nothing at all for a value that is not a URL, such as the raw connection string duck_lineage sends when an attach string has no host.
func cleanURI(raw string) string {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || u.Scheme == "" || strings.ContainsAny(raw, " \t") || strings.Contains(strings.ToLower(raw), "password") {
		return ""
	}
	if u.Host == "" && u.Scheme != "file" {
		return ""
	}
	out := url.URL{Scheme: u.Scheme, Host: u.Host, Path: u.Path}
	return out.String()
}

// facetMap returns one facet as a map, or nil when the facet is absent, malformed or marked deleted.
func facetMap(facets map[string]interface{}, key string) map[string]interface{} {
	if facets == nil {
		return nil
	}
	m, ok := facets[key].(map[string]interface{})
	if !ok {
		return nil
	}
	if deleted, _ := m["_deleted"].(bool); deleted {
		return nil
	}
	return m
}

// integrationProviders maps the jobType facet's integration onto Marmot provider names.
var integrationProviders = map[string]string{
	"AIRFLOW":   ProviderAirflow,
	"SPARK":     ProviderSpark,
	"DBT":       ProviderDBT,
	"FLINK":     "Flink",
	"TRINO":     "Trino",
	"DAGSTER":   "Dagster",
	"DUCKDB":    "DuckDB",
	"PREFECT":   "Prefect",
	"HIVE":      "Hive",
	"GLUE":      "Glue",
	"SNOWFLAKE": "Snowflake",
	"BIGQUERY":  ProviderBigQuery,
}

// producerProviders maps a fragment of the producer URL onto a provider, checked in order.
var producerProviders = []struct{ fragment, provider string }{
	{"airflow", ProviderAirflow},
	{"spark", ProviderSpark},
	{"dbt", ProviderDBT},
	{"flink", "Flink"},
	{"trino", "Trino"},
	{"dagster", "Dagster"},
	{"duck_lineage", "DuckDB"},
	{"prefect", "Prefect"},
}

// jobProvider names the system that ran a job: the jobType facet's integration when it names one Marmot knows, otherwise the producer.
func jobProvider(event *RunEvent) string {
	if jt := facetMap(event.Job.Facets, "jobType"); jt != nil {
		if integration, _ := jt["integration"].(string); integration != "" {
			if p, ok := integrationProviders[strings.ToUpper(integration)]; ok {
				return p
			}
		}
	}
	producer := strings.ToLower(event.Producer)
	for _, p := range producerProviders {
		if strings.Contains(producer, p.fragment) {
			return p.provider
		}
	}
	return ProviderOpenLineage
}

// identifyJob works out the asset an OpenLineage job is. The MRN carries the whole job name, so dag_a.load and dag_b.load stay apart; the asset shows the last part of it.
func identifyJob(event *RunEvent) jobIdentity {
	provider := jobProvider(event)
	assetType := inferJobType(event, provider)
	id := jobIdentity{
		Provider: provider,
		Type:     assetType,
		Name:     extractSimpleJobName(event.Job.Name),
		MRN:      openLineageMRN(assetType, provider, event.Job.Namespace, event.Job.Name),
	}
	// dbt names every data test <node>.test, so the last part alone would call each one test.
	if assetType == AssetTypeTest && strings.HasSuffix(event.Job.Name, ".test") {
		parts := strings.Split(event.Job.Name, ".")
		if len(parts) >= 2 {
			id.Name = parts[len(parts)-2] + ".test"
		}
	}
	legacyProvider := legacyInferProvider(event.Producer)
	legacy := openLineageMRN(inferJobType(event, legacyProvider), legacyProvider, event.Job.Namespace, extractSimpleJobName(event.Job.Name))
	if legacy != id.MRN {
		id.Legacy = []string{legacy}
	}
	return id
}

// legacyDatasetMRNs lists the MRNs earlier releases gave a dataset: the 0.13 form, which turned an address namespace into a host hint, and the form before it, which put the whole namespace in the name. Both used the provider and type the old substring rules inferred.
func legacyDatasetMRNs(ds *Dataset) []string {
	provider := legacyInferDatasetProvider(ds)
	assetType := legacyInferDatasetType(ds)
	out := []string{openLineageDatasetMRN(assetType, provider, ds.Namespace, ds.Name)}
	if old := openLineageMRN(assetType, provider, ds.Namespace, ds.Name); !slices.Contains(out, old) {
		out = append(out, old)
	}
	return out
}

// legacyInferProvider is how releases up to 0.13 named the system behind a job, kept so the MRNs they produced can be found again.
func legacyInferProvider(producer string) string {
	producer = strings.ToLower(producer)
	switch {
	case strings.Contains(producer, "airflow"):
		return ProviderAirflow
	case strings.Contains(producer, "spark"):
		return ProviderSpark
	case strings.Contains(producer, "dbt"):
		return ProviderDBT
	}
	return ProviderOpenLineage
}

// legacyInferDatasetProvider is how releases up to 0.13 guessed a dataset's provider from words in its namespace. It is kept for namespaces with no mapping and to find the MRNs those releases produced.
func legacyInferDatasetProvider(dataset *Dataset) string {
	namespace := strings.ToLower(dataset.Namespace)
	switch {
	case strings.Contains(namespace, "bigquery") || strings.Contains(namespace, "bq"):
		return ProviderBigQuery
	case strings.Contains(namespace, "postgres"):
		return ProviderPostgreSQL
	case strings.Contains(namespace, "mysql"):
		return ProviderMySQL
	case strings.Contains(namespace, "sqlserver"):
		return ProviderSQLServer
	case strings.Contains(namespace, "kafka"):
		return ProviderKafka
	case strings.Contains(namespace, "s3"):
		return ProviderS3
	case strings.Contains(namespace, "gcs"):
		return ProviderGCS
	case strings.Contains(namespace, "azure"):
		return ProviderAzure
	}
	return ProviderOpenLineage
}

// legacyInferDatasetType is how releases up to 0.13 guessed a dataset's type, kept alongside legacyInferDatasetProvider.
func legacyInferDatasetType(dataset *Dataset) string {
	namespace := strings.ToLower(dataset.Namespace)
	name := strings.ToLower(dataset.Name)

	if strings.Contains(namespace, "postgres") || strings.Contains(namespace, "mysql") ||
		strings.Contains(namespace, "sqlserver") || strings.Contains(namespace, "bigquery") {
		return AssetTypeTable
	}
	if strings.Contains(namespace, "kafka") {
		return AssetTypeTopic
	}
	if strings.Contains(namespace, "s3") || strings.Contains(namespace, "gcs") || strings.Contains(namespace, "azure") {
		if strings.Contains(name, ".parquet") || strings.Contains(name, ".csv") {
			return AssetTypeFile
		}
		return AssetTypeBucket
	}
	if strings.Contains(name, ".parquet") || strings.Contains(name, ".csv") {
		return AssetTypeFile
	}
	return AssetTypeDataset
}
