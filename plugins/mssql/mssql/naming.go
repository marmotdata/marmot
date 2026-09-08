package mssql

import (
	"regexp"
	"strconv"
	"strings"

	"github.com/marmotdata/plugin-sdk/mrn"
)

// providerName is the exact provider string Marmot uses for SQL Server. It
// contains a space, which mrn.New lowercases and dashes to "sql-server".
const providerName = "SQL Server"

// assetMRN is the single place a SQL Server MRN is built. Every pass (assets,
// foreign keys, view references, statistics) goes through it, so the passes
// cannot drift into addressing the same object differently.
func assetMRN(assetType, name string) string {
	return mrn.New(assetType, providerName, name)
}

// qualifiedName is the name every table, view and function asset carries. One
// SQL Server instance holds many databases and two databases can hold the same
// schema and object name, so the name is fully qualified to stay unique.
func qualifiedName(database, schema, object string) string {
	return database + "." + schema + "." + object
}

// quoteIdent wraps an identifier in brackets so names containing spaces,
// keywords or punctuation interpolate safely. A closing bracket inside the
// name is doubled, which is how SQL Server escapes it.
func quoteIdent(name string) string {
	return "[" + strings.ReplaceAll(name, "]", "]]") + "]"
}

// Types whose declared size SQL Server reports in bytes that equal characters.
var byteLengthTypes = map[string]bool{
	"char": true, "varchar": true, "binary": true, "varbinary": true,
}

// Types stored as UTF-16, so sys.columns.max_length is twice the declared
// character length.
var doubleByteLengthTypes = map[string]bool{
	"nchar": true, "nvarchar": true,
}

var precisionScaleTypes = map[string]bool{
	"decimal": true, "numeric": true,
}

var scaleOnlyTypes = map[string]bool{
	"datetime2": true, "time": true, "datetimeoffset": true,
}

// renderDataType writes a column's type the way SQL Server would declare it:
// varchar(50), nvarchar(max), decimal(10,2), datetime2(7). Types that carry no
// size are returned bare. maxLength of -1 means one of the MAX types.
func renderDataType(typeName string, maxLength int64, precision, scale int) string {
	name := strings.ToLower(typeName)

	switch {
	case byteLengthTypes[name]:
		if maxLength == -1 {
			return name + "(max)"
		}
		return name + "(" + strconv.FormatInt(maxLength, 10) + ")"

	case doubleByteLengthTypes[name]:
		if maxLength == -1 {
			return name + "(max)"
		}
		return name + "(" + strconv.FormatInt(maxLength/2, 10) + ")"

	case precisionScaleTypes[name]:
		return name + "(" + strconv.Itoa(precision) + "," + strconv.Itoa(scale) + ")"

	case scaleOnlyTypes[name]:
		return name + "(" + strconv.Itoa(scale) + ")"
	}

	return name
}

// identPattern matches one part of a SQL Server object name, either bracket
// quoted or bare. Bare names may start with @ or # for variables and temp
// tables, which are matched so they can be recognised and dropped.
const identPattern = `(?:\[[^\]]*\]|[A-Za-z_@#][\w$@#]*)`

var (
	// Matches the object named directly after FROM or JOIN, allowing up to
	// four dot-separated parts. A trailing "(" is captured so table-valued
	// function calls can be told apart from plain object references.
	viewRefRE = regexp.MustCompile(`(?is)\b(?:from|join)\s+(` + identPattern + `(?:\s*\.\s*` + identPattern + `?){0,3})\s*(\()?`)

	// Matches the name a common table expression binds, either the first one
	// after WITH or a later one after a comma.
	cteRE = regexp.MustCompile(`(?is)(?:\bwith\b|,)\s*(` + identPattern + `)\s*(?:\([^()]*\)\s*)?as\s*\(`)

	lineCommentRE  = regexp.MustCompile(`(?m)--[^\n]*`)
	blockCommentRE = regexp.MustCompile(`(?s)/\*.*?\*/`)
	stringLitRE    = regexp.MustCompile(`'(?:[^']|'')*'`)
)

// extractViewReferences returns the objects a view definition reads from, each
// as a fully qualified "database.schema.object" name. Names in the definition
// may be written with one, two or three parts, bracket quoted or not, so the
// missing parts are filled from the view's own database and schema.
//
// This is a text scan, not a parser: it deliberately skips common table
// expression names, table variables, temp tables, function calls and anything
// named through a linked server, because none of those are objects this run
// discovered. Callers resolve the result against the objects they found and
// drop the rest, so an over-eager match costs nothing.
func extractViewReferences(definition, defaultDatabase, defaultSchema string) []string {
	sql := stripNoise(definition)

	cteNames := make(map[string]bool)
	for _, m := range cteRE.FindAllStringSubmatch(sql, -1) {
		cteNames[strings.ToLower(unquoteIdent(m[1]))] = true
	}

	var refs []string
	seen := make(map[string]bool)

	for _, m := range viewRefRE.FindAllStringSubmatch(sql, -1) {
		// A "(" right after the name makes it a table-valued function call.
		if m[2] == "(" {
			continue
		}

		parts := splitQualifiedName(m[1])
		if parts == nil {
			continue
		}

		if len(parts) == 1 && cteNames[strings.ToLower(parts[0])] {
			continue
		}

		var database, schema, object string
		switch len(parts) {
		case 1:
			database, schema, object = defaultDatabase, defaultSchema, parts[0]
		case 2:
			database, schema, object = defaultDatabase, parts[0], parts[1]
		case 3:
			database, schema, object = parts[0], parts[1], parts[2]
		default:
			// A four-part name addresses a linked server, which is outside
			// this instance.
			continue
		}

		if schema == "" {
			schema = defaultSchema
		}
		if database == "" || object == "" {
			continue
		}

		name := qualifiedName(database, schema, object)
		if seen[strings.ToLower(name)] {
			continue
		}
		seen[strings.ToLower(name)] = true
		refs = append(refs, name)
	}

	return refs
}

// stripNoise removes comments and string literals so their contents cannot be
// mistaken for object references. Literals become empty quotes rather than
// disappearing, to keep the surrounding tokens apart.
func stripNoise(sql string) string {
	sql = blockCommentRE.ReplaceAllString(sql, " ")
	sql = lineCommentRE.ReplaceAllString(sql, " ")
	return stringLitRE.ReplaceAllString(sql, "''")
}

// splitQualifiedName breaks a dotted name into its parts and unwraps brackets.
// It returns nil for names that cannot refer to a discovered object: table
// variables (@name) and temp tables (#name).
func splitQualifiedName(name string) []string {
	raw := strings.Split(name, ".")
	parts := make([]string, 0, len(raw))
	for _, p := range raw {
		parts = append(parts, unquoteIdent(strings.TrimSpace(p)))
	}

	last := parts[len(parts)-1]
	if last == "" {
		return nil
	}
	if strings.HasPrefix(last, "@") || strings.HasPrefix(last, "#") {
		return nil
	}
	return parts
}

// unquoteIdent removes the brackets around an identifier and undoubles any
// escaped closing bracket, the inverse of quoteIdent.
func unquoteIdent(part string) string {
	if strings.HasPrefix(part, "[") && strings.HasSuffix(part, "]") && len(part) >= 2 {
		return strings.ReplaceAll(part[1:len(part)-1], "]]", "]")
	}
	return part
}
