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

// assetMRN is the single place an MRN is built. The asset pass and the
// lineage pass both go through it so the two can never drift into addressing
// the same object differently.
func assetMRN(assetType, provider, name string) string {
	return mrn.New(assetType, provider, name)
}

// tableMRN addresses a table or view. Athena tables are named by their bare
// name under the Glue provider, exactly as plugins/glue names them.
func tableMRN(assetType, table string) string {
	return assetMRN(assetType, providerGlue, table)
}

func databaseMRN(database string) string {
	return assetMRN(typeDatabase, providerGlue, database)
}

func workGroupMRN(workGroup string) string {
	return assetMRN(typeWorkGroup, providerAthena, workGroup)
}

func savedQueryMRN(workGroup, name string) string {
	return assetMRN(typeSavedQuery, providerAthena, savedQueryName(workGroup, name))
}

func catalogMRN(catalog string) string {
	return assetMRN(typeCatalog, providerAthena, catalog)
}

func bucketMRN(bucket string) string {
	return assetMRN(typeBucket, providerS3, bucket)
}

// savedQueryName qualifies a saved query by its workgroup, because two
// workgroups can each hold a query of the same name.
func savedQueryName(workGroup, name string) string {
	if workGroup == "" {
		return name
	}
	return workGroup + "/" + name
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
