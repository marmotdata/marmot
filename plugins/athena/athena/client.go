package athena

import (
	"context"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go-v2/service/athena"
	athenatypes "github.com/aws/aws-sdk-go-v2/service/athena/types"
	"github.com/aws/aws-sdk-go-v2/service/glue"
	gluetypes "github.com/aws/aws-sdk-go-v2/service/glue/types"
	"github.com/rs/zerolog/log"
)

// maxPages stops a broken or hostile paginator from looping forever. At the
// page sizes Athena and Glue use this is far more than any real account holds.
const maxPages = 200

// athenaAPI is the part of the Athena API this plugin calls. Discovery takes
// the interface rather than *athena.Client so tests can drive it with a fake.
type athenaAPI interface {
	ListDataCatalogs(ctx context.Context, in *athena.ListDataCatalogsInput, opts ...func(*athena.Options)) (*athena.ListDataCatalogsOutput, error)
	GetDataCatalog(ctx context.Context, in *athena.GetDataCatalogInput, opts ...func(*athena.Options)) (*athena.GetDataCatalogOutput, error)
	ListDatabases(ctx context.Context, in *athena.ListDatabasesInput, opts ...func(*athena.Options)) (*athena.ListDatabasesOutput, error)
	ListTableMetadata(ctx context.Context, in *athena.ListTableMetadataInput, opts ...func(*athena.Options)) (*athena.ListTableMetadataOutput, error)
	ListWorkGroups(ctx context.Context, in *athena.ListWorkGroupsInput, opts ...func(*athena.Options)) (*athena.ListWorkGroupsOutput, error)
	GetWorkGroup(ctx context.Context, in *athena.GetWorkGroupInput, opts ...func(*athena.Options)) (*athena.GetWorkGroupOutput, error)
	ListNamedQueries(ctx context.Context, in *athena.ListNamedQueriesInput, opts ...func(*athena.Options)) (*athena.ListNamedQueriesOutput, error)
	BatchGetNamedQuery(ctx context.Context, in *athena.BatchGetNamedQueryInput, opts ...func(*athena.Options)) (*athena.BatchGetNamedQueryOutput, error)
	GetNamedQuery(ctx context.Context, in *athena.GetNamedQueryInput, opts ...func(*athena.Options)) (*athena.GetNamedQueryOutput, error)
	ListTagsForResource(ctx context.Context, in *athena.ListTagsForResourceInput, opts ...func(*athena.Options)) (*athena.ListTagsForResourceOutput, error)
}

// glueAPI is the part of the Glue API this plugin calls, used when the Athena
// metadata API cannot answer for a Glue-backed catalog.
type glueAPI interface {
	GetDatabases(ctx context.Context, in *glue.GetDatabasesInput, opts ...func(*glue.Options)) (*glue.GetDatabasesOutput, error)
	GetTables(ctx context.Context, in *glue.GetTablesInput, opts ...func(*glue.Options)) (*glue.GetTablesOutput, error)
}

// The real SDK clients have to keep satisfying the interfaces above, so an
// SDK upgrade that changes a signature fails the build here rather than at
// the call site.
var (
	_ athenaAPI = (*athena.Client)(nil)
	_ glueAPI   = (*glue.Client)(nil)
)

// catalogDatabase and catalogTable are the shapes discovery works with. Both
// the Athena metadata API and the Glue API are mapped into them so the asset
// building code does not care which one answered.
type catalogDatabase struct {
	Name        string
	Description string
	Parameters  map[string]string
	LocationURI string
}

type catalogTable struct {
	Name           string
	TableType      string
	CreateTime     string
	LastAccessTime string
	Columns        []catalogColumn
	PartitionKeys  []catalogColumn
	// Parameters uses the Athena metadata API's key names (location,
	// inputformat, outputformat, serde.serialization.lib, comment, ...). The
	// Glue reader fills the same keys from the storage descriptor.
	Parameters map[string]string
}

type catalogColumn struct {
	Name    string
	Type    string
	Comment string
}

// catalogReader lists the databases and tables of one Athena data catalog.
type catalogReader interface {
	// name reports which API the reader talks to, for logs and the report.
	name() string
	databases(ctx context.Context, catalog string) ([]catalogDatabase, error)
	tables(ctx context.Context, catalog, database string) ([]catalogTable, error)
}

// athenaReader reads through the Athena metadata API. It works for every
// catalog type, including federated ones Glue knows nothing about.
type athenaReader struct {
	client athenaAPI
}

func (r athenaReader) name() string { return "athena" }

func (r athenaReader) databases(ctx context.Context, catalog string) ([]catalogDatabase, error) {
	var out []catalogDatabase
	var token *string

	for page := 0; page < maxPages; page++ {
		resp, err := r.client.ListDatabases(ctx, &athena.ListDatabasesInput{
			CatalogName: &catalog,
			NextToken:   token,
		})
		if err != nil {
			return nil, fmt.Errorf("listing databases in catalog %s: %w", catalog, err)
		}

		for _, db := range resp.DatabaseList {
			out = append(out, catalogDatabase{
				Name:        deref(db.Name),
				Description: deref(db.Description),
				Parameters:  db.Parameters,
			})
		}

		if resp.NextToken == nil || *resp.NextToken == "" {
			return out, nil
		}
		token = resp.NextToken
	}

	log.Warn().Str("catalog", catalog).Int("pages", maxPages).Msg("Stopped listing databases at the page cap")
	return out, nil
}

func (r athenaReader) tables(ctx context.Context, catalog, database string) ([]catalogTable, error) {
	var out []catalogTable
	var token *string

	for page := 0; page < maxPages; page++ {
		resp, err := r.client.ListTableMetadata(ctx, &athena.ListTableMetadataInput{
			CatalogName:  &catalog,
			DatabaseName: &database,
			NextToken:    token,
		})
		if err != nil {
			return nil, fmt.Errorf("listing table metadata in %s.%s: %w", catalog, database, err)
		}

		for _, t := range resp.TableMetadataList {
			out = append(out, catalogTable{
				Name:           deref(t.Name),
				TableType:      deref(t.TableType),
				CreateTime:     formatTime(t.CreateTime),
				LastAccessTime: formatTime(t.LastAccessTime),
				Columns:        athenaColumns(t.Columns),
				PartitionKeys:  athenaColumns(t.PartitionKeys),
				Parameters:     t.Parameters,
			})
		}

		if resp.NextToken == nil || *resp.NextToken == "" {
			return out, nil
		}
		token = resp.NextToken
	}

	log.Warn().Str("catalog", catalog).Str("database", database).Int("pages", maxPages).
		Msg("Stopped listing tables at the page cap")
	return out, nil
}

func athenaColumns(cols []athenatypes.Column) []catalogColumn {
	out := make([]catalogColumn, 0, len(cols))
	for _, c := range cols {
		out = append(out, catalogColumn{
			Name:    deref(c.Name),
			Type:    deref(c.Type),
			Comment: deref(c.Comment),
		})
	}
	return out
}

// glueReader reads the Glue Data Catalog directly. A GLUE-type Athena catalog
// is the Glue Data Catalog, so the two APIs describe the same objects; this
// path exists for deployments where the Athena metadata API is unavailable.
type glueReader struct {
	client glueAPI
}

func (r glueReader) name() string { return "glue" }

func (r glueReader) databases(ctx context.Context, catalog string) ([]catalogDatabase, error) {
	var out []catalogDatabase
	var token *string

	for page := 0; page < maxPages; page++ {
		resp, err := r.client.GetDatabases(ctx, &glue.GetDatabasesInput{NextToken: token})
		if err != nil {
			return nil, fmt.Errorf("getting Glue databases: %w", err)
		}

		for _, db := range resp.DatabaseList {
			out = append(out, catalogDatabase{
				Name:        deref(db.Name),
				Description: deref(db.Description),
				Parameters:  db.Parameters,
				LocationURI: deref(db.LocationUri),
			})
		}

		if resp.NextToken == nil || *resp.NextToken == "" {
			return out, nil
		}
		token = resp.NextToken
	}

	log.Warn().Str("catalog", catalog).Int("pages", maxPages).Msg("Stopped getting Glue databases at the page cap")
	return out, nil
}

func (r glueReader) tables(ctx context.Context, catalog, database string) ([]catalogTable, error) {
	var out []catalogTable
	var token *string

	for page := 0; page < maxPages; page++ {
		resp, err := r.client.GetTables(ctx, &glue.GetTablesInput{
			DatabaseName: &database,
			NextToken:    token,
		})
		if err != nil {
			return nil, fmt.Errorf("getting Glue tables in %s: %w", database, err)
		}

		for _, t := range resp.TableList {
			out = append(out, glueTable(t))
		}

		if resp.NextToken == nil || *resp.NextToken == "" {
			return out, nil
		}
		token = resp.NextToken
	}

	log.Warn().Str("catalog", catalog).Str("database", database).Int("pages", maxPages).
		Msg("Stopped getting Glue tables at the page cap")
	return out, nil
}

// glueTable folds a Glue table into the same shape the Athena metadata API
// returns, including the parameter keys Athena synthesises from the storage
// descriptor, so one asset builder serves both readers.
func glueTable(t gluetypes.Table) catalogTable {
	params := make(map[string]string, len(t.Parameters)+6)
	for k, v := range t.Parameters {
		params[k] = v
	}

	if sd := t.StorageDescriptor; sd != nil {
		putIfSet(params, "location", deref(sd.Location))
		putIfSet(params, "inputformat", deref(sd.InputFormat))
		putIfSet(params, "outputformat", deref(sd.OutputFormat))
		if sd.SerdeInfo != nil {
			putIfSet(params, "serde.serialization.lib", deref(sd.SerdeInfo.SerializationLibrary))
		}
		if sd.Compressed {
			params["compressed"] = "true"
		}
	}
	putIfSet(params, "comment", deref(t.Description))
	putIfSet(params, "view_original_text", deref(t.ViewOriginalText))

	var columns []catalogColumn
	if t.StorageDescriptor != nil {
		columns = glueColumns(t.StorageDescriptor.Columns)
	}

	return catalogTable{
		Name:           deref(t.Name),
		TableType:      deref(t.TableType),
		CreateTime:     formatTime(t.CreateTime),
		LastAccessTime: formatTime(t.LastAccessTime),
		Columns:        columns,
		PartitionKeys:  glueColumns(t.PartitionKeys),
		Parameters:     params,
	}
}

func glueColumns(cols []gluetypes.Column) []catalogColumn {
	out := make([]catalogColumn, 0, len(cols))
	for _, c := range cols {
		out = append(out, catalogColumn{
			Name:    deref(c.Name),
			Type:    deref(c.Type),
			Comment: deref(c.Comment),
		})
	}
	return out
}

// listCatalogs returns every data catalog Athena reports. Real Athena always
// has AwsDataCatalog, so an empty list means the account has nothing beyond
// the default and the caller falls back to it.
func (s *Source) listCatalogs(ctx context.Context) ([]catalogSummary, error) {
	var out []catalogSummary
	var token *string

	for page := 0; page < maxPages; page++ {
		resp, err := s.athena.ListDataCatalogs(ctx, &athena.ListDataCatalogsInput{NextToken: token})
		if err != nil {
			return nil, fmt.Errorf("listing data catalogs: %w", err)
		}

		for _, c := range resp.DataCatalogsSummary {
			out = append(out, catalogSummary{
				Name: deref(c.CatalogName),
				Type: string(c.Type),
			})
		}

		if resp.NextToken == nil || *resp.NextToken == "" {
			return out, nil
		}
		token = resp.NextToken
	}

	log.Warn().Int("pages", maxPages).Msg("Stopped listing data catalogs at the page cap")
	return out, nil
}

type catalogSummary struct {
	Name        string
	Type        string
	Description string
	Parameters  map[string]string
}

// describeCatalog fills in the description and parameters ListDataCatalogs
// leaves out. A catalog that cannot be described is still worth cataloguing,
// so a failure only costs the extra detail.
func (s *Source) describeCatalog(ctx context.Context, c catalogSummary) catalogSummary {
	resp, err := s.athena.GetDataCatalog(ctx, &athena.GetDataCatalogInput{Name: &c.Name})
	if err != nil {
		log.Warn().Err(err).Str("catalog", c.Name).Msg("Failed to describe data catalog")
		return c
	}
	if resp.DataCatalog == nil {
		return c
	}

	c.Description = deref(resp.DataCatalog.Description)
	c.Parameters = resp.DataCatalog.Parameters
	if t := string(resp.DataCatalog.Type); t != "" {
		c.Type = t
	}
	return c
}

// listWorkGroups returns every workgroup with its full configuration.
func (s *Source) listWorkGroups(ctx context.Context) ([]athenatypes.WorkGroup, error) {
	var names []string
	var token *string

	for page := 0; page < maxPages; page++ {
		resp, err := s.athena.ListWorkGroups(ctx, &athena.ListWorkGroupsInput{NextToken: token})
		if err != nil {
			return nil, fmt.Errorf("listing workgroups: %w", err)
		}

		for _, wg := range resp.WorkGroups {
			names = append(names, deref(wg.Name))
		}

		if resp.NextToken == nil || *resp.NextToken == "" {
			break
		}
		token = resp.NextToken
		if page == maxPages-1 {
			log.Warn().Int("pages", maxPages).Msg("Stopped listing workgroups at the page cap")
		}
	}

	var out []athenatypes.WorkGroup
	for _, name := range names {
		if name == "" || !s.wantWorkGroup(name) {
			continue
		}
		resp, err := s.athena.GetWorkGroup(ctx, &athena.GetWorkGroupInput{WorkGroup: &name})
		if err != nil {
			log.Warn().Err(err).Str("workgroup", name).Msg("Failed to describe workgroup")
			continue
		}
		if resp.WorkGroup == nil {
			continue
		}
		out = append(out, *resp.WorkGroup)
	}

	return out, nil
}

// listNamedQueries returns the saved queries of one workgroup. Athena has no
// call that lists them across workgroups, so the caller iterates workgroups.
func (s *Source) listNamedQueries(ctx context.Context, workGroup string) ([]athenatypes.NamedQuery, error) {
	var ids []string
	var token *string

	for page := 0; page < maxPages; page++ {
		resp, err := s.athena.ListNamedQueries(ctx, &athena.ListNamedQueriesInput{
			WorkGroup: &workGroup,
			NextToken: token,
		})
		if err != nil {
			return nil, fmt.Errorf("listing named queries in workgroup %s: %w", workGroup, err)
		}

		ids = append(ids, resp.NamedQueryIds...)

		if resp.NextToken == nil || *resp.NextToken == "" {
			break
		}
		token = resp.NextToken
		if page == maxPages-1 {
			log.Warn().Str("workgroup", workGroup).Int("pages", maxPages).
				Msg("Stopped listing named queries at the page cap")
		}
	}

	return s.fetchNamedQueries(ctx, ids)
}

// namedQueryBatchSize is the maximum number of ids BatchGetNamedQuery accepts.
const namedQueryBatchSize = 50

// fetchNamedQueries resolves query ids to their definitions. It prefers the
// batch call and falls back to one call per id when the endpoint does not
// implement it, which is the case for some Athena-compatible endpoints.
func (s *Source) fetchNamedQueries(ctx context.Context, ids []string) ([]athenatypes.NamedQuery, error) {
	var out []athenatypes.NamedQuery

	for start := 0; start < len(ids); start += namedQueryBatchSize {
		end := min(start+namedQueryBatchSize, len(ids))
		batch := ids[start:end]

		resp, err := s.athena.BatchGetNamedQuery(ctx, &athena.BatchGetNamedQueryInput{
			NamedQueryIds: batch,
		})
		if err != nil {
			log.Debug().Err(err).Msg("BatchGetNamedQuery unavailable, fetching saved queries one at a time")
			one, oneErr := s.fetchNamedQueriesIndividually(ctx, batch)
			if oneErr != nil {
				return nil, oneErr
			}
			out = append(out, one...)
			continue
		}

		out = append(out, resp.NamedQueries...)
	}

	return out, nil
}

func (s *Source) fetchNamedQueriesIndividually(ctx context.Context, ids []string) ([]athenatypes.NamedQuery, error) {
	var out []athenatypes.NamedQuery

	for _, id := range ids {
		resp, err := s.athena.GetNamedQuery(ctx, &athena.GetNamedQueryInput{NamedQueryId: &id})
		if err != nil {
			log.Warn().Err(err).Str("named_query_id", id).Msg("Failed to get saved query")
			continue
		}
		if resp.NamedQuery == nil {
			continue
		}
		out = append(out, *resp.NamedQuery)
	}

	return out, nil
}

// resourceTags reads the tags of a workgroup or data catalog. Athena answers
// with an error when a resource has no tags at all, so a failure here is
// logged at debug level and treated as "no tags".
func (s *Source) resourceTags(ctx context.Context, arn string) map[string]string {
	if arn == "" {
		return nil
	}

	resp, err := s.athena.ListTagsForResource(ctx, &athena.ListTagsForResourceInput{ResourceARN: &arn})
	if err != nil {
		log.Debug().Err(err).Str("resource", arn).Msg("No tags read for resource")
		return nil
	}

	tags := make(map[string]string, len(resp.Tags))
	for _, t := range resp.Tags {
		tags[deref(t.Key)] = deref(t.Value)
	}
	return tags
}

func putIfSet(m map[string]string, key, value string) {
	if value != "" {
		m[key] = value
	}
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// arnPartition returns the ARN partition for a region. Athena does not return
// workgroup or catalog ARNs, so the plugin builds them to read tags.
func arnPartition(region string) string {
	switch {
	case strings.HasPrefix(region, "cn-"):
		return "aws-cn"
	case strings.HasPrefix(region, "us-gov-"):
		return "aws-us-gov"
	case strings.HasPrefix(region, "us-iso-"):
		return "aws-iso"
	case strings.HasPrefix(region, "us-isob-"):
		return "aws-iso-b"
	default:
		return "aws"
	}
}

func athenaARN(region, accountID, resource string) string {
	if region == "" || accountID == "" {
		return ""
	}
	return fmt.Sprintf("arn:%s:athena:%s:%s:%s", arnPartition(region), region, accountID, resource)
}
