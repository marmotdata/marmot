package openmetadata

import (
	"fmt"
	"strings"
)

// The structs below mirror the parts of OpenMetadata's entity JSON that
// Marmot uses. Every OpenMetadata entity shares the same envelope
// (id, name, fullyQualifiedName, service, serviceType, tags, owners),
// so entityBase is embedded in each one.

// entityRef is OpenMetadata's EntityReference: the pointer shape used
// for services, parents, owners, domains and lineage nodes.
type entityRef struct {
	ID                 string `json:"id"`
	Type               string `json:"type"`
	Name               string `json:"name"`
	FullyQualifiedName string `json:"fullyQualifiedName"`
	DisplayName        string `json:"displayName"`
	Description        string `json:"description"`
	Deleted            bool   `json:"deleted"`
}

// tagLabel is a tag or glossary term assigned to an entity. Source is
// "Classification" for tags and "Glossary" for glossary terms.
type tagLabel struct {
	TagFQN      string `json:"tagFQN"`
	Source      string `json:"source"`
	LabelType   string `json:"labelType"`
	State       string `json:"state"`
	Description string `json:"description"`
}

// entityBase holds the fields every OpenMetadata entity carries.
type entityBase struct {
	ID                 string      `json:"id"`
	Name               string      `json:"name"`
	DisplayName        string      `json:"displayName"`
	FullyQualifiedName string      `json:"fullyQualifiedName"`
	Description        string      `json:"description"`
	Deleted            bool        `json:"deleted"`
	UpdatedAt          int64       `json:"updatedAt"`
	Version            float64     `json:"version"`
	Href               string      `json:"href"`
	SourceURL          string      `json:"sourceUrl"`
	Service            entityRef   `json:"service"`
	ServiceType        string      `json:"serviceType"`
	Tags               []tagLabel  `json:"tags"`
	Owners             []entityRef `json:"owners"`
	Domains            []entityRef `json:"domains"`
	DataProducts       []entityRef `json:"dataProducts"`
}

// column is a table, data model or search index column.
type column struct {
	Name            string     `json:"name"`
	DataType        string     `json:"dataType"`
	DataTypeDisplay string     `json:"dataTypeDisplay"`
	DataLength      int        `json:"dataLength"`
	Description     string     `json:"description"`
	Constraint      string     `json:"constraint"`
	OrdinalPosition int        `json:"ordinalPosition"`
	Tags            []tagLabel `json:"tags"`
	Children        []column   `json:"children"`
}

type database struct {
	entityBase
	Default bool `json:"default"`
}

type databaseSchema struct {
	entityBase
	Database entityRef `json:"database"`
}

type table struct {
	entityBase
	TableType        string        `json:"tableType"`
	Columns          []column      `json:"columns"`
	Database         entityRef     `json:"database"`
	DatabaseSchema   entityRef     `json:"databaseSchema"`
	TableConstraints []constraint  `json:"tableConstraints"`
	Profile          *tableProfile `json:"profile"`
	UsageSummary     *usageSummary `json:"usageSummary"`
}

type constraint struct {
	ConstraintType string   `json:"constraintType"`
	Columns        []string `json:"columns"`
}

type tableProfile struct {
	RowCount    float64 `json:"rowCount"`
	ColumnCount float64 `json:"columnCount"`
	SizeInByte  float64 `json:"sizeInByte"`
}

type usageSummary struct {
	DailyStats   usageStats `json:"dailyStats"`
	WeeklyStats  usageStats `json:"weeklyStats"`
	MonthlyStats usageStats `json:"monthlyStats"`
}

type usageStats struct {
	Count          float64 `json:"count"`
	PercentileRank float64 `json:"percentileRank"`
}

type storedProcedure struct {
	entityBase
	Database            entityRef            `json:"database"`
	DatabaseSchema      entityRef            `json:"databaseSchema"`
	StoredProcedureType string               `json:"storedProcedureType"`
	Code                *storedProcedureCode `json:"storedProcedureCode"`
}

type storedProcedureCode struct {
	Language string `json:"language"`
	Code     string `json:"code"`
}

type topic struct {
	entityBase
	Partitions         int            `json:"partitions"`
	ReplicationFactor  int            `json:"replicationFactor"`
	RetentionSize      float64        `json:"retentionSize"`
	RetentionTime      float64        `json:"retentionTime"`
	MaximumMessageSize int            `json:"maximumMessageSize"`
	CleanupPolicies    []string       `json:"cleanupPolicies"`
	MessageSchema      *messageSchema `json:"messageSchema"`
}

type messageSchema struct {
	SchemaText   string   `json:"schemaText"`
	SchemaType   string   `json:"schemaType"`
	SchemaFields []column `json:"schemaFields"`
}

type container struct {
	entityBase
	Parent          *entityRef          `json:"parent"`
	Children        []entityRef         `json:"children"`
	Prefix          string              `json:"prefix"`
	NumberOfObjects float64             `json:"numberOfObjects"`
	Size            float64             `json:"size"`
	FileFormats     []string            `json:"fileFormats"`
	DataModel       *containerDataModel `json:"dataModel"`
}

type containerDataModel struct {
	IsPartitioned bool     `json:"isPartitioned"`
	Columns       []column `json:"columns"`
}

type dashboard struct {
	entityBase
	DashboardType string      `json:"dashboardType"`
	Charts        []entityRef `json:"charts"`
	DataModels    []entityRef `json:"dataModels"`
	Project       string      `json:"project"`
}

type chart struct {
	entityBase
	ChartType  string      `json:"chartType"`
	Dashboards []entityRef `json:"dashboards"`
}

type dashboardDataModel struct {
	entityBase
	DataModelType string   `json:"dataModelType"`
	SQL           string   `json:"sql"`
	Columns       []column `json:"columns"`
	Project       string   `json:"project"`
}

type pipeline struct {
	entityBase
	Tasks            []pipelineTask `json:"tasks"`
	ScheduleInterval string         `json:"scheduleInterval"`
	Concurrency      int            `json:"concurrency"`
	PipelineLocation string         `json:"pipelineLocation"`
}

type pipelineTask struct {
	Name            string      `json:"name"`
	DisplayName     string      `json:"displayName"`
	Description     string      `json:"description"`
	SourceURL       string      `json:"sourceUrl"`
	TaskType        string      `json:"taskType"`
	DownstreamTasks []string    `json:"downstreamTasks"`
	Tags            []tagLabel  `json:"tags"`
	Owners          []entityRef `json:"owners"`
}

// pipelineStatus is one execution of a pipeline. OpenMetadata calls the
// identifier executionId; runId is what its older releases and some
// connectors send.
type pipelineStatus struct {
	ExecutionStatus string       `json:"executionStatus"`
	Timestamp       int64        `json:"timestamp"`
	ExecutionID     string       `json:"executionId"`
	RunID           string       `json:"runId"`
	TaskStatus      []taskStatus `json:"taskStatus"`
}

// identifier is the run's own id, or a stable one derived from when it
// ran: OpenMetadata does not require an execution to carry one, and
// Marmot groups run events by it.
func (s pipelineStatus) identifier(pipelineName string) string {
	if s.ExecutionID != "" {
		return s.ExecutionID
	}
	if s.RunID != "" {
		return s.RunID
	}
	return fmt.Sprintf("%s-%d", pipelineName, s.Timestamp)
}

type taskStatus struct {
	Name            string `json:"name"`
	ExecutionStatus string `json:"executionStatus"`
	StartTime       int64  `json:"startTime"`
	EndTime         int64  `json:"endTime"`
	LogLink         string `json:"logLink"`
}

type mlModel struct {
	entityBase
	Algorithm     string             `json:"algorithm"`
	Target        string             `json:"target"`
	Server        string             `json:"server"`
	Dashboard     *entityRef         `json:"dashboard"`
	MlFeatures    []mlFeature        `json:"mlFeatures"`
	MlHyperParams []mlHyperParameter `json:"mlHyperParameters"`
	MlStore       *mlStore           `json:"mlStore"`
}

type mlFeature struct {
	Name             string            `json:"name"`
	DataType         string            `json:"dataType"`
	Description      string            `json:"description"`
	FeatureSources   []mlFeatureSource `json:"featureSources"`
	FeatureAlgorithm string            `json:"featureAlgorithm"`
}

type mlFeatureSource struct {
	Name       string     `json:"name"`
	DataType   string     `json:"dataType"`
	DataSource *entityRef `json:"dataSource"`
}

type mlHyperParameter struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

type mlStore struct {
	Storage         string `json:"storage"`
	ImageRepository string `json:"imageRepository"`
}

type searchIndex struct {
	entityBase
	IndexType string   `json:"indexType"`
	Fields    []column `json:"fields"`
}

// driveService is the drive itself: My Drive, a shared drive, or a
// SharePoint site. OpenMetadata models it as a service, but unlike a
// database connection it is a real container people navigate, so it is
// catalogued rather than skipped.
type driveService struct {
	entityBase
}

// Drive entities. OpenMetadata models a drive as
// service > directory > file, and separately
// service > spreadsheet > worksheet, where a worksheet is a sheet of
// columns and so the only drive entity with a schema.
type directory struct {
	entityBase
	DirectoryType string     `json:"directoryType"`
	Path          string     `json:"path"`
	IsShared      bool       `json:"isShared"`
	Parent        *entityRef `json:"parent"`
}

type driveFile struct {
	entityBase
	FileType      string     `json:"fileType"`
	FileExtension string     `json:"fileExtension"`
	MimeType      string     `json:"mimeType"`
	Path          string     `json:"path"`
	Size          float64    `json:"size"`
	FileVersion   string     `json:"fileVersion"`
	Checksum      string     `json:"checksum"`
	IsShared      bool       `json:"isShared"`
	WebViewLink   string     `json:"webViewLink"`
	Directory     *entityRef `json:"directory"`
}

type spreadsheet struct {
	entityBase
	Path      string     `json:"path"`
	Size      float64    `json:"size"`
	Directory *entityRef `json:"directory"`
}

type worksheet struct {
	entityBase
	Spreadsheet *entityRef `json:"spreadsheet"`
	Columns     []column   `json:"columns"`
	IsHidden    bool       `json:"isHidden"`
	RowCount    float64    `json:"rowCount"`
}

type apiCollection struct {
	entityBase
	EndpointURL string `json:"endpointURL"`
}

type apiEndpoint struct {
	entityBase
	APICollection entityRef `json:"apiCollection"`
	EndpointURL   string    `json:"endpointURL"`
	RequestMethod string    `json:"requestMethod"`
}

// glossary is a named vocabulary. OpenMetadata names a glossary by its
// name alone, so its fully qualified name is the prefix every term
// below it carries.
type glossary struct {
	entityBase
}

// glossaryTerm is one business definition. Glossary holds the vocabulary
// it belongs to, and Parent the term it sits under, which is absent for
// a term at the top of its glossary.
type glossaryTerm struct {
	entityBase
	Glossary     entityRef   `json:"glossary"`
	Parent       *entityRef  `json:"parent"`
	Synonyms     []string    `json:"synonyms"`
	RelatedTerms []entityRef `json:"relatedTerms"`
}

// listResponse is the envelope every OpenMetadata list endpoint returns.
type listResponse[T any] struct {
	Data   []T `json:"data"`
	Paging struct {
		After  string `json:"after"`
		Before string `json:"before"`
		Total  int    `json:"total"`
	} `json:"paging"`
}

// lineageResponse is the shape of GET /api/v1/lineage/{type}/{id}.
type lineageResponse struct {
	Entity          entityRef     `json:"entity"`
	Nodes           []entityRef   `json:"nodes"`
	UpstreamEdges   []lineageEdge `json:"upstreamEdges"`
	DownstreamEdges []lineageEdge `json:"downstreamEdges"`
}

type lineageEdge struct {
	FromEntity string          `json:"fromEntity"`
	ToEntity   string          `json:"toEntity"`
	Details    *lineageDetails `json:"lineageDetails"`
}

type lineageDetails struct {
	SQLQuery string     `json:"sqlQuery"`
	Source   string     `json:"source"`
	Pipeline *entityRef `json:"pipeline"`
}

// splitFQN splits an OpenMetadata fully qualified name into its parts.
// Name parts containing a dot are quoted in the FQN, so a plain
// strings.Split would break them apart: sample_data."my.db".shopify has
// three parts, not four.
func splitFQN(fqn string) []string {
	var (
		parts   []string
		current strings.Builder
		quoted  bool
	)

	for _, r := range fqn {
		switch {
		case r == '"':
			quoted = !quoted
		case r == '.' && !quoted:
			parts = append(parts, current.String())
			current.Reset()
		default:
			current.WriteRune(r)
		}
	}
	parts = append(parts, current.String())

	return parts
}

// fqnBelowService returns the FQN parts below the service component,
// which is always the first part of an OpenMetadata FQN.
func fqnBelowService(fqn string) []string {
	parts := splitFQN(fqn)
	if len(parts) <= 1 {
		return nil
	}
	return parts[1:]
}
