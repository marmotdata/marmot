package athena

import (
	"encoding/base64"
	"encoding/json"
	"regexp"
	"strings"
	"time"

	"github.com/marmotdata/plugin-sdk/mrn"
)

// Provider strings are exact and case sensitive.
//
// Athena's own objects are filed under Athena. The tables it queries are not:
// Athena keeps no catalog of its own, it reads the Glue Data Catalog, which
// plugins/glue already files under Glue. Sharing the provider makes an Athena
// run and a Glue run land on one asset instead of two half-populated ones.
const (
	providerAthena = "Athena"
	providerGlue   = "Glue"
	providerS3     = "S3"
)

const (
	typeTable      = "Table"
	typeView       = "View"
	typeDatabase   = "Database"
	typeWorkGroup  = "WorkGroup"
	typeSavedQuery = "Data Model Object"
	typeCatalog    = "Catalog"
	typeBucket     = "Bucket"
)

// hiveViewTableType is the table type Athena reports for a view.
const hiveViewTableType = "VIRTUAL_VIEW"

// glueCatalogPath is the catalog level an asset path starts with. The default catalog is the account's Glue Data Catalog, which plugins/glue files without a catalog level, so its databases and tables leave the level out and both plugins land on one asset. Any other catalog is named in the path.
func glueCatalogPath(catalog string) []string {
	if catalog == "" || catalog == defaultCatalogName {
		return nil
	}
	return []string{catalog}
}

// tableMRN addresses a table or view under the Glue provider. The asset pass and the lineage pass both go through it so the two can never drift into addressing the same object differently.
func (s *Source) tableMRN(assetType, catalog, database, table string) string {
	path := append(glueCatalogPath(catalog), database, table)
	return s.config.AssetMRN(assetType, providerGlue, path...)
}

func (s *Source) databaseMRN(catalog, database string) string {
	path := append(glueCatalogPath(catalog), database)
	return s.config.AssetMRN(typeDatabase, providerGlue, path...)
}

func (s *Source) workGroupMRN(workGroup string) string {
	return s.config.AssetMRN(typeWorkGroup, providerAthena, workGroup)
}

func (s *Source) savedQueryMRN(workGroup, name string) string {
	return s.config.AssetMRN(typeSavedQuery, providerAthena, savedQueryPath(workGroup, name)...)
}

func (s *Source) catalogMRN(catalog string) string {
	return s.config.AssetMRN(typeCatalog, providerAthena, catalog)
}

// bucketMRN points at a bucket the S3 plugin owns. S3 names are global to the account, so no zone applies.
func bucketMRN(bucket string) string {
	return mrn.Asset(typeBucket, providerS3, bucket).String()
}

// savedQueryPath qualifies a saved query by its workgroup, because two workgroups can each hold a query of the same name.
func savedQueryPath(workGroup, name string) []string {
	if workGroup == "" {
		return []string{name}
	}
	return []string{workGroup, name}
}

// previousSavedQueryMRN is the MRN earlier releases gave a saved query, which folded the workgroup into one name.
func previousSavedQueryMRN(workGroup, name string) string {
	if workGroup != "" {
		name = workGroup + "/" + name
	}
	return mrn.Asset(typeSavedQuery, providerAthena, name).LegacyString()
}

// tableAssetType maps an Athena table type to a Marmot asset type. Athena
// reports views as VIRTUAL_VIEW; everything else is a table.
func tableAssetType(tableType string) string {
	if strings.EqualFold(tableType, hiveViewTableType) {
		return typeView
	}
	return typeTable
}

// prestoViewPattern matches the wrapper Athena puts around a view definition:
// /* Presto View: <base64 of a JSON document holding the SQL> */
var prestoViewPattern = regexp.MustCompile(`(?s)/\*\s*Presto\s+View:?\s*([A-Za-z0-9+/=\s]+?)\s*\*/`)

// decodeViewText returns the SQL behind a stored view definition. Athena
// wraps the SQL in a base64 encoded JSON document; text that is not in that
// form is already SQL and is returned unchanged.
func decodeViewText(raw string) string {
	match := prestoViewPattern.FindStringSubmatch(raw)
	if match == nil {
		return strings.TrimSpace(raw)
	}

	encoded := strings.Join(strings.Fields(match[1]), "")
	decoded, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return strings.TrimSpace(raw)
	}

	var view struct {
		OriginalSQL string `json:"originalSql"`
	}
	if err := json.Unmarshal(decoded, &view); err != nil || view.OriginalSQL == "" {
		return strings.TrimSpace(raw)
	}

	return strings.TrimSpace(view.OriginalSQL)
}

// tableRef is a table named inside a query. Database is empty when the query
// wrote the table name unqualified.
type tableRef struct {
	Database string
	Table    string
}

var (
	commentPattern    = regexp.MustCompile(`(?s)/\*.*?\*/|--[^\n]*`)
	tableRefPattern   = regexp.MustCompile("(?i)\\b(?:from|join)\\s+((?:\"[^\"]+\"|`[^`]+`|[A-Za-z_][A-Za-z0-9_$]*)(?:\\.(?:\"[^\"]+\"|`[^`]+`|[A-Za-z_][A-Za-z0-9_$]*))*)")
	identifierPattern = regexp.MustCompile("\"[^\"]+\"|`[^`]+`|[A-Za-z_][A-Za-z0-9_$]*")
)

// extractQueryTables lists the tables a query reads, taken from what follows
// FROM and JOIN. It is deliberately shallow: a name that is not a table this
// run discovered is dropped when the lineage edge is built, so a false
// positive costs nothing.
func extractQueryTables(query string) []tableRef {
	stripped := commentPattern.ReplaceAllString(query, " ")

	var refs []tableRef
	seen := make(map[tableRef]struct{})

	for _, match := range tableRefPattern.FindAllStringSubmatch(stripped, -1) {
		parts := identifierPattern.FindAllString(match[1], -1)
		if len(parts) == 0 {
			continue
		}
		for i := range parts {
			parts[i] = unquoteIdent(parts[i])
		}

		ref := tableRef{Table: parts[len(parts)-1]}
		if len(parts) > 1 {
			// A three part name is catalog.database.table, so the database is
			// always the part before the table.
			ref.Database = parts[len(parts)-2]
		}
		if ref.Table == "" {
			continue
		}
		if _, ok := seen[ref]; ok {
			continue
		}
		seen[ref] = struct{}{}
		refs = append(refs, ref)
	}

	return refs
}

func unquoteIdent(s string) string {
	if len(s) >= 2 {
		if (s[0] == '"' && s[len(s)-1] == '"') || (s[0] == '`' && s[len(s)-1] == '`') {
			return s[1 : len(s)-1]
		}
	}
	return s
}

// s3Bucket returns the bucket a table location points at, or an empty string
// when the location is not on S3. Athena writes s3://, and Hive style tables
// can carry the older s3a:// and s3n:// schemes.
func s3Bucket(location string) string {
	for _, scheme := range []string{"s3://", "s3a://", "s3n://"} {
		rest, ok := strings.CutPrefix(location, scheme)
		if !ok {
			continue
		}
		bucket, _, _ := strings.Cut(rest, "/")
		return bucket
	}
	return ""
}

func formatTime(t *time.Time) string {
	if t == nil || t.IsZero() {
		return ""
	}
	return t.Format(time.RFC3339)
}

// workGroupConsoleURL deep links to a workgroup in the Athena console.
func workGroupConsoleURL(region, workGroup string) string {
	if region == "" || workGroup == "" {
		return ""
	}
	return "https://" + region + ".console.aws.amazon.com/athena/home?region=" + region +
		"#/workgroups/details/" + workGroup
}
