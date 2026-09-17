package starrocks

import (
	"regexp"
	"strconv"
	"strings"
)

// tableDDL is what SHOW CREATE TABLE tells us that information_schema
// does not: the key model, partitioning, distribution, sort order and
// table properties. StarRocks exposes none of these through a queryable
// view, so they are parsed from the DDL text.
type tableDDL struct {
	KeyModel   string
	KeyColumns []string

	PartitionType       string
	PartitionColumns    []string
	PartitionExpression string

	Distribution        string
	DistributionColumns []string
	Buckets             int

	OrderBy []string

	Properties map[string]string

	AutoIncrementColumns []string
	ForeignKeys          []foreignKey
}

// foreignKey is one entry of the foreign_key_constraints table property.
// StarRocks does not enforce it, but it declares which table a column
// points at, which is exactly the lineage we want.
type foreignKey struct {
	Columns           []string
	ReferencedCatalog string
	ReferencedDB      string
	ReferencedTable   string
	ReferencedColumns []string
}

var (
	keyClauseRe      = regexp.MustCompile(`(?m)^\s*(DUPLICATE|AGGREGATE|UNIQUE|PRIMARY) KEY\s*\(([^)]*)\)`)
	partitionRe      = regexp.MustCompile(`(?m)^\s*PARTITION BY\s+(.+?)\s*$`)
	distributionRe   = regexp.MustCompile(`(?m)^\s*DISTRIBUTED BY\s+(HASH\s*\(([^)]*)\)|RANDOM)(?:\s+BUCKETS\s+(\d+))?`)
	orderByRe        = regexp.MustCompile(`(?m)^\s*ORDER BY\s*\(([^)]*)\)`)
	propertyRe       = regexp.MustCompile(`"([^"]+)"\s*=\s*"([^"]*)"`)
	autoIncrementRe  = regexp.MustCompile("(?m)^\\s*`([^`]+)`[^\\n]*\\bAUTO_INCREMENT\\b")
	foreignKeyRe     = regexp.MustCompile(`\(([^)]+)\)\s*REFERENCES\s+([^\s(]+)\s*\(([^)]+)\)`)
	functionCallRe   = regexp.MustCompile(`^\w+\s*\((.*)\)$`)
	bareIdentifierRe = regexp.MustCompile("^`?[A-Za-z_][A-Za-z0-9_]*`?$")
	// Credential property names are dotted for external catalogs
	// ("aws.s3.secret_key"), so the name is matched as a whole: anything
	// mentioning a password, secret, token or credential, plus anything
	// ending in a key. "foreign_key_constraints" ends in "constraints",
	// so it survives and can still be read for lineage.
	credentialRe = regexp.MustCompile(`(?i)"([^"]*(?:password|secret|token|credential)[^"]*|[^"]*[._]key)"\s*=\s*"[^"]*"`)
)

// parseTableDDL reads the clauses of a SHOW CREATE TABLE (or
// MATERIALIZED VIEW) statement. Anything it cannot find is left empty;
// callers only emit what is set.
func parseTableDDL(ddl string) tableDDL {
	var d tableDDL

	if m := keyClauseRe.FindStringSubmatch(ddl); m != nil {
		d.KeyModel = m[1]
		d.KeyColumns = splitIdentifiers(m[2])
	}

	if m := partitionRe.FindStringSubmatch(ddl); m != nil {
		d.PartitionType, d.PartitionColumns, d.PartitionExpression = parsePartitionClause(m[1])
	}

	if m := distributionRe.FindStringSubmatch(ddl); m != nil {
		if strings.HasPrefix(strings.ToUpper(m[1]), "HASH") {
			d.Distribution = "HASH"
			d.DistributionColumns = splitIdentifiers(m[2])
		} else {
			d.Distribution = "RANDOM"
		}
		if m[3] != "" {
			d.Buckets, _ = strconv.Atoi(m[3])
		}
	}

	if m := orderByRe.FindStringSubmatch(ddl); m != nil {
		d.OrderBy = splitIdentifiers(m[1])
	}

	d.Properties = parseProperties(ddl)
	d.AutoIncrementColumns = parseAutoIncrementColumns(ddl)
	d.ForeignKeys = parseForeignKeys(d.Properties["foreign_key_constraints"])

	return d
}

// parsePartitionClause classifies the text after PARTITION BY. StarRocks
// prints one of:
//
//	RANGE(`dt`)            explicit range partitions follow on later lines
//	LIST(`region`)(        explicit list partitions follow
//	(`dt`, `city`)         expression partitioning by column
//	date_trunc('day', dt)  expression partitioning by function
func parsePartitionClause(clause string) (partitionType string, columns []string, expression string) {
	upper := strings.ToUpper(clause)

	switch {
	case strings.HasPrefix(upper, "RANGE"):
		return "RANGE", columnsInParens(clause), ""
	case strings.HasPrefix(upper, "LIST"):
		return "LIST", columnsInParens(clause), ""
	case strings.HasPrefix(clause, "("):
		return "EXPRESSION", columnsInParens(clause), ""
	}

	// A function call: the columns are whichever arguments are bare
	// identifiers, so date_trunc('month', dt) yields dt.
	if m := functionCallRe.FindStringSubmatch(clause); m != nil {
		for _, arg := range strings.Split(m[1], ",") {
			arg = strings.TrimSpace(arg)
			if bareIdentifierRe.MatchString(arg) {
				columns = append(columns, strings.Trim(arg, "`"))
			}
		}
	}
	return "EXPRESSION", columns, clause
}

// columnsInParens returns the identifiers inside the first (...) group.
func columnsInParens(s string) []string {
	start := strings.Index(s, "(")
	if start < 0 {
		return nil
	}
	end := strings.Index(s[start:], ")")
	if end < 0 {
		return nil
	}
	return splitIdentifiers(s[start+1 : start+end])
}

// splitIdentifiers turns "`a`, `b`" into [a b].
func splitIdentifiers(list string) []string {
	var out []string
	for _, part := range strings.Split(list, ",") {
		part = strings.TrimSpace(strings.Trim(strings.TrimSpace(part), "`"))
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}

// parseProperties reads the "key" = "value" pairs of the PROPERTIES block.
// Only the text after PROPERTIES is scanned so column comments that happen
// to contain quotes are not mistaken for properties.
func parseProperties(ddl string) map[string]string {
	idx := strings.LastIndex(ddl, "PROPERTIES")
	if idx < 0 {
		return nil
	}

	props := make(map[string]string)
	for _, m := range propertyRe.FindAllStringSubmatch(ddl[idx:], -1) {
		props[m[1]] = m[2]
	}
	if len(props) == 0 {
		return nil
	}
	return props
}

// parseAutoIncrementColumns finds columns declared AUTO_INCREMENT. Neither
// SHOW FULL COLUMNS nor information_schema.columns reports the flag, so
// the DDL is the only place it shows.
func parseAutoIncrementColumns(ddl string) []string {
	var cols []string
	for _, m := range autoIncrementRe.FindAllStringSubmatch(ddl, -1) {
		cols = append(cols, m[1])
	}
	return cols
}

// parseForeignKeys reads a foreign_key_constraints property value such as
// "(customer_id) REFERENCES default_catalog.shop.customers(customer_id);
// (sku) REFERENCES products(sku)".
func parseForeignKeys(value string) []foreignKey {
	var keys []foreignKey
	for _, m := range foreignKeyRe.FindAllStringSubmatch(value, -1) {
		fk := foreignKey{
			Columns:           splitIdentifiers(m[1]),
			ReferencedColumns: splitIdentifiers(m[3]),
		}
		parts := splitQualifiedName(m[2])
		switch len(parts) {
		case 3:
			fk.ReferencedCatalog, fk.ReferencedDB, fk.ReferencedTable = parts[0], parts[1], parts[2]
		case 2:
			fk.ReferencedDB, fk.ReferencedTable = parts[0], parts[1]
		case 1:
			fk.ReferencedTable = parts[0]
		default:
			continue
		}
		keys = append(keys, fk)
	}
	return keys
}

// splitQualifiedName splits catalog.db.table on dots, dropping backticks.
func splitQualifiedName(name string) []string {
	var parts []string
	for _, p := range strings.Split(name, ".") {
		p = strings.Trim(strings.TrimSpace(p), "`")
		if p != "" {
			parts = append(parts, p)
		}
	}
	return parts
}

// redactDDL blanks credential-looking property values. StarRocks already
// hides the password of MySQL external tables, but other engines carry
// keys and secrets in PROPERTIES and a catalog is no place for them.
func redactDDL(ddl string) string {
	return credentialRe.ReplaceAllString(ddl, `"$1" = "***"`)
}

// viewQuery returns the SELECT part of a CREATE VIEW or CREATE
// MATERIALIZED VIEW statement, which is what belongs in Asset.Query. The
// full statement stays in metadata.
func viewQuery(ddl string) string {
	m := viewQueryRe.FindStringSubmatch(ddl)
	if m == nil {
		return ""
	}
	return strings.TrimSuffix(strings.TrimSpace(m[1]), ";")
}

var viewQueryRe = regexp.MustCompile(`(?is)\bAS\s+((?:SELECT|WITH|\()\s*.*)$`)
