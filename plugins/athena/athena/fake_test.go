package athena

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/athena"
	athenatypes "github.com/aws/aws-sdk-go-v2/service/athena/types"
	"github.com/aws/aws-sdk-go-v2/service/glue"
	gluetypes "github.com/aws/aws-sdk-go-v2/service/glue/types"
	pluginsdk "github.com/marmotdata/plugin-sdk"
	"github.com/stretchr/testify/require"
)

// errNotImplemented stands in for an Athena compatible endpoint that does not
// serve an operation, which is how the plugin decides to fall back.
var errNotImplemented = errors.New("operation not implemented by this endpoint")

// fakeAthena serves the Athena responses the plugin reads, in the shapes the
// real API returns.
type fakeAthena struct {
	catalogs      []athenatypes.DataCatalogSummary
	catalogDetail map[string]athenatypes.DataCatalog
	databases     map[string][]athenatypes.Database
	tables        map[string][]athenatypes.TableMetadata
	workGroups    []athenatypes.WorkGroup
	namedQueries  map[string][]athenatypes.NamedQuery
	tags          map[string][]athenatypes.Tag

	// tableMetadataErr makes ListTableMetadata fail, as it does on an
	// endpoint that never implemented it.
	tableMetadataErr error
	// batchErr makes BatchGetNamedQuery fail, forcing the per-id path.
	batchErr error

	getNamedQueryCalls int
}

func (f *fakeAthena) ListDataCatalogs(ctx context.Context, in *athena.ListDataCatalogsInput, opts ...func(*athena.Options)) (*athena.ListDataCatalogsOutput, error) {
	return &athena.ListDataCatalogsOutput{DataCatalogsSummary: f.catalogs}, nil
}

func (f *fakeAthena) GetDataCatalog(ctx context.Context, in *athena.GetDataCatalogInput, opts ...func(*athena.Options)) (*athena.GetDataCatalogOutput, error) {
	catalog, ok := f.catalogDetail[aws.ToString(in.Name)]
	if !ok {
		return &athena.GetDataCatalogOutput{}, nil
	}
	return &athena.GetDataCatalogOutput{DataCatalog: &catalog}, nil
}

func (f *fakeAthena) ListDatabases(ctx context.Context, in *athena.ListDatabasesInput, opts ...func(*athena.Options)) (*athena.ListDatabasesOutput, error) {
	return &athena.ListDatabasesOutput{DatabaseList: f.databases[aws.ToString(in.CatalogName)]}, nil
}

func (f *fakeAthena) ListTableMetadata(ctx context.Context, in *athena.ListTableMetadataInput, opts ...func(*athena.Options)) (*athena.ListTableMetadataOutput, error) {
	if f.tableMetadataErr != nil {
		return nil, f.tableMetadataErr
	}
	key := aws.ToString(in.CatalogName) + "/" + aws.ToString(in.DatabaseName)
	return &athena.ListTableMetadataOutput{TableMetadataList: f.tables[key]}, nil
}

func (f *fakeAthena) ListWorkGroups(ctx context.Context, in *athena.ListWorkGroupsInput, opts ...func(*athena.Options)) (*athena.ListWorkGroupsOutput, error) {
	summaries := make([]athenatypes.WorkGroupSummary, 0, len(f.workGroups))
	for _, wg := range f.workGroups {
		summaries = append(summaries, athenatypes.WorkGroupSummary{Name: wg.Name, State: wg.State})
	}
	return &athena.ListWorkGroupsOutput{WorkGroups: summaries}, nil
}

func (f *fakeAthena) GetWorkGroup(ctx context.Context, in *athena.GetWorkGroupInput, opts ...func(*athena.Options)) (*athena.GetWorkGroupOutput, error) {
	for _, wg := range f.workGroups {
		if aws.ToString(wg.Name) == aws.ToString(in.WorkGroup) {
			return &athena.GetWorkGroupOutput{WorkGroup: &wg}, nil
		}
	}
	return &athena.GetWorkGroupOutput{}, nil
}

func (f *fakeAthena) ListNamedQueries(ctx context.Context, in *athena.ListNamedQueriesInput, opts ...func(*athena.Options)) (*athena.ListNamedQueriesOutput, error) {
	var ids []string
	for _, q := range f.namedQueries[aws.ToString(in.WorkGroup)] {
		ids = append(ids, aws.ToString(q.NamedQueryId))
	}
	return &athena.ListNamedQueriesOutput{NamedQueryIds: ids}, nil
}

func (f *fakeAthena) BatchGetNamedQuery(ctx context.Context, in *athena.BatchGetNamedQueryInput, opts ...func(*athena.Options)) (*athena.BatchGetNamedQueryOutput, error) {
	if f.batchErr != nil {
		return nil, f.batchErr
	}
	var found []athenatypes.NamedQuery
	for _, id := range in.NamedQueryIds {
		if q, ok := f.queryByID(id); ok {
			found = append(found, q)
		}
	}
	return &athena.BatchGetNamedQueryOutput{NamedQueries: found}, nil
}

func (f *fakeAthena) GetNamedQuery(ctx context.Context, in *athena.GetNamedQueryInput, opts ...func(*athena.Options)) (*athena.GetNamedQueryOutput, error) {
	f.getNamedQueryCalls++
	q, ok := f.queryByID(aws.ToString(in.NamedQueryId))
	if !ok {
		return &athena.GetNamedQueryOutput{}, nil
	}
	return &athena.GetNamedQueryOutput{NamedQuery: &q}, nil
}

func (f *fakeAthena) ListTagsForResource(ctx context.Context, in *athena.ListTagsForResourceInput, opts ...func(*athena.Options)) (*athena.ListTagsForResourceOutput, error) {
	tags, ok := f.tags[aws.ToString(in.ResourceARN)]
	if !ok {
		return nil, errors.New("resource has no tags")
	}
	return &athena.ListTagsForResourceOutput{Tags: tags}, nil
}

func (f *fakeAthena) queryByID(id string) (athenatypes.NamedQuery, bool) {
	for _, queries := range f.namedQueries {
		for _, q := range queries {
			if aws.ToString(q.NamedQueryId) == id {
				return q, true
			}
		}
	}
	return athenatypes.NamedQuery{}, false
}

// fakeGlue serves the Glue Data Catalog responses the fallback path reads.
type fakeGlue struct {
	databases []gluetypes.Database
	tables    map[string][]gluetypes.Table
}

func (f *fakeGlue) GetDatabases(ctx context.Context, in *glue.GetDatabasesInput, opts ...func(*glue.Options)) (*glue.GetDatabasesOutput, error) {
	return &glue.GetDatabasesOutput{DatabaseList: f.databases}, nil
}

func (f *fakeGlue) GetTables(ctx context.Context, in *glue.GetTablesInput, opts ...func(*glue.Options)) (*glue.GetTablesOutput, error) {
	return &glue.GetTablesOutput{TableList: f.tables[aws.ToString(in.DatabaseName)]}, nil
}

// seedAthena is the fixture the discovery tests run against: a Glue catalog
// holding a shop database with two tables and a view, plus an analytics
// workgroup with one saved query.
func seedAthena() *fakeAthena {
	created := time.Date(2026, 3, 1, 9, 0, 0, 0, time.UTC)

	return &fakeAthena{
		catalogs: []athenatypes.DataCatalogSummary{
			{CatalogName: aws.String("AwsDataCatalog"), Type: athenatypes.DataCatalogTypeGlue},
			{CatalogName: aws.String("hive-lab"), Type: athenatypes.DataCatalogTypeHive},
		},
		catalogDetail: map[string]athenatypes.DataCatalog{
			"AwsDataCatalog": {
				Name:        aws.String("AwsDataCatalog"),
				Type:        athenatypes.DataCatalogTypeGlue,
				Description: aws.String("The account Glue Data Catalog"),
				Parameters:  map[string]string{"catalog-id": "123456789012"},
			},
		},
		databases: map[string][]athenatypes.Database{
			"AwsDataCatalog": {
				{Name: aws.String("shop"), Description: aws.String("Storefront tables")},
			},
		},
		tables: map[string][]athenatypes.TableMetadata{
			"AwsDataCatalog/shop": {
				{
					Name:       aws.String("orders"),
					TableType:  aws.String("EXTERNAL_TABLE"),
					CreateTime: &created,
					Columns: []athenatypes.Column{
						{Name: aws.String("order_id"), Type: aws.String("bigint"), Comment: aws.String("Order identifier")},
						{Name: aws.String("items"), Type: aws.String("array<struct<sku:string,qty:int>>")},
					},
					PartitionKeys: []athenatypes.Column{
						{Name: aws.String("dt"), Type: aws.String("string"), Comment: aws.String("Ingest date")},
					},
					Parameters: map[string]string{
						"location":                "s3://marmot-lake/orders/",
						"classification":          "parquet",
						"comment":                 "One row per order",
						"inputformat":             "org.apache.hadoop.hive.ql.io.parquet.MapredParquetInputFormat",
						"outputformat":            "org.apache.hadoop.hive.ql.io.parquet.MapredParquetOutputFormat",
						"serde.serialization.lib": "org.apache.hadoop.hive.ql.io.parquet.serde.ParquetHiveSerDe",
						"projection.enabled":      "true",
					},
				},
				{
					Name:      aws.String("customers"),
					TableType: aws.String("EXTERNAL_TABLE"),
					Columns: []athenatypes.Column{
						{Name: aws.String("customer_id"), Type: aws.String("bigint")},
					},
					Parameters: map[string]string{"location": "s3://marmot-lake/customers/"},
				},
				{
					Name:      aws.String("order_totals"),
					TableType: aws.String("VIRTUAL_VIEW"),
					Columns: []athenatypes.Column{
						{Name: aws.String("total"), Type: aws.String("double")},
					},
					Parameters: map[string]string{
						"view_original_text": prestoViewText(`SELECT sum(total) FROM shop.orders`),
					},
				},
			},
		},
		workGroups: []athenatypes.WorkGroup{{
			Name:         aws.String("analytics"),
			State:        athenatypes.WorkGroupStateEnabled,
			Description:  aws.String("Analyst queries"),
			CreationTime: &created,
			Configuration: &athenatypes.WorkGroupConfiguration{
				EnforceWorkGroupConfiguration:   aws.Bool(true),
				PublishCloudWatchMetricsEnabled: aws.Bool(true),
				RequesterPaysEnabled:            aws.Bool(false),
				BytesScannedCutoffPerQuery:      aws.Int64(10_000_000),
				EngineVersion: &athenatypes.EngineVersion{
					EffectiveEngineVersion: aws.String("Athena engine version 3"),
					SelectedEngineVersion:  aws.String("AUTO"),
				},
				ResultConfiguration: &athenatypes.ResultConfiguration{
					OutputLocation: aws.String("s3://marmot-results/analytics/"),
					EncryptionConfiguration: &athenatypes.EncryptionConfiguration{
						EncryptionOption: athenatypes.EncryptionOptionSseS3,
					},
				},
			},
		}},
		namedQueries: map[string][]athenatypes.NamedQuery{
			"analytics": {{
				NamedQueryId: aws.String("nq-1"),
				Name:         aws.String("daily-revenue"),
				Description:  aws.String("Revenue per day"),
				Database:     aws.String("shop"),
				WorkGroup:    aws.String("analytics"),
				QueryString:  aws.String("SELECT c.name, sum(o.total) FROM orders o JOIN customers c ON c.customer_id = o.customer_id GROUP BY 1"),
			}},
		},
	}
}

// seedGlue mirrors seedAthena's shop database through the Glue API, for the
// tests that exercise the fallback path.
func seedGlue() *fakeGlue {
	created := time.Date(2026, 3, 1, 9, 0, 0, 0, time.UTC)

	return &fakeGlue{
		databases: []gluetypes.Database{{
			Name:        aws.String("shop"),
			Description: aws.String("Storefront tables"),
			LocationUri: aws.String("s3://marmot-lake/shop/"),
		}},
		tables: map[string][]gluetypes.Table{
			"shop": {{
				Name:        aws.String("orders"),
				TableType:   aws.String("EXTERNAL_TABLE"),
				Description: aws.String("One row per order"),
				CreateTime:  &created,
				StorageDescriptor: &gluetypes.StorageDescriptor{
					Location:    aws.String("s3://marmot-lake/orders/"),
					InputFormat: aws.String("org.apache.hadoop.hive.ql.io.parquet.MapredParquetInputFormat"),
					SerdeInfo: &gluetypes.SerDeInfo{
						SerializationLibrary: aws.String("org.apache.hadoop.hive.ql.io.parquet.serde.ParquetHiveSerDe"),
					},
					Columns: []gluetypes.Column{
						{Name: aws.String("order_id"), Type: aws.String("bigint")},
					},
				},
				PartitionKeys: []gluetypes.Column{
					{Name: aws.String("dt"), Type: aws.String("string")},
				},
				Parameters: map[string]string{"classification": "parquet"},
			}},
		},
	}
}

// prestoViewText wraps SQL the way Athena stores a view definition: a base64
// encoded JSON document inside a comment marker.
func prestoViewText(sql string) string {
	document, err := json.Marshal(map[string]any{
		"originalSql": sql,
		"catalog":     "awsdatacatalog",
		"schema":      "shop",
	})
	if err != nil {
		panic(err)
	}
	return "/* Presto View: " + base64.StdEncoding.EncodeToString(document) + " */"
}

// newTestSource builds a Source wired to the fakes, with the defaults a real
// run would have applied.
func newTestSource(t *testing.T, athenaClient athenaAPI, glueClient glueAPI, raw pluginsdk.RawConfig) *Source {
	t.Helper()

	s := &Source{}
	_, err := s.Validate(raw)
	require.NoError(t, err)

	s.athena = athenaClient
	s.glue = glueClient
	s.region = "us-east-1"
	return s
}
