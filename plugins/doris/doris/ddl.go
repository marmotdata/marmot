package doris

import (
	"regexp"
	"strconv"
	"strings"
)

// tableDDL is what discovery reads out of a SHOW CREATE TABLE statement.
// Doris keeps the table model, partitioning and distribution only in the
// DDL, so this is the one place they can be read from.
type tableDDL struct {
	Engine              string
	KeyModel            string
	KeyColumns          []string
	PartitionType       string
	PartitionColumns    []string
	DistributionType    string
	DistributionColumns []string
	Buckets             int
	AutoBucket          bool
	Properties          map[string]string
}

var (
	engineRe       = regexp.MustCompile(`(?im)^\)\s*ENGINE\s*=\s*([A-Za-z_]+)`)
	keyModelRe     = regexp.MustCompile(`(?im)^(DUPLICATE|AGGREGATE|UNIQUE)\s+KEY\s*\(`)
	partitionRe    = regexp.MustCompile(`(?im)^(?:AUTO\s+)?PARTITION\s+BY\s+(RANGE|LIST)\s*\(`)
	distributionRe = regexp.MustCompile(`(?im)^DISTRIBUTED\s+BY\s+(HASH|RANDOM)\s*(\()?`)
	bucketsRe      = regexp.MustCompile(`(?i)\bBUCKETS\s+(\d+|AUTO)\b`)
	propertyRe     = regexp.MustCompile(`"([^"]+)"\s*=\s*"([^"]*)"`)
)

// parseTableDDL reads the table model, key columns, partitioning,
// distribution and properties out of a SHOW CREATE TABLE statement. Anything
// it cannot find is left empty; the caller decides what that means.
func parseTableDDL(ddl string) tableDDL {
	var parsed tableDDL

	if m := engineRe.FindStringSubmatch(ddl); m != nil {
		parsed.Engine = m[1]
	}

	if loc := keyModelRe.FindStringSubmatchIndex(ddl); loc != nil {
		parsed.KeyModel = strings.ToUpper(ddl[loc[2]:loc[3]])
		if group, ok := parenGroup(ddl, loc[1]-1); ok {
			parsed.KeyColumns = splitIdentifiers(group)
		}
	}

	if loc := partitionRe.FindStringSubmatchIndex(ddl); loc != nil {
		parsed.PartitionType = strings.ToUpper(ddl[loc[2]:loc[3]])
		if group, ok := parenGroup(ddl, loc[1]-1); ok {
			parsed.PartitionColumns = splitIdentifiers(group)
		}
	}

	if loc := distributionRe.FindStringSubmatchIndex(ddl); loc != nil {
		parsed.DistributionType = strings.ToUpper(ddl[loc[2]:loc[3]])
		rest := ddl[loc[1]:]
		if loc[4] >= 0 {
			if group, ok := parenGroup(ddl, loc[4]); ok {
				parsed.DistributionColumns = splitIdentifiers(group)
				rest = ddl[loc[4]+len(group)+2:]
			}
		}
		// Only the same DDL line carries BUCKETS; a later line could
		// belong to a rollup or property.
		if nl := strings.IndexByte(rest, '\n'); nl >= 0 {
			rest = rest[:nl]
		}
		if m := bucketsRe.FindStringSubmatch(rest); m != nil {
			if strings.EqualFold(m[1], "AUTO") {
				parsed.AutoBucket = true
			} else {
				parsed.Buckets, _ = strconv.Atoi(m[1])
			}
		}
	}

	parsed.Properties = parseProperties(ddl)

	return parsed
}

// parseProperties reads the "key" = "value" pairs in the PROPERTIES block
// at the end of a SHOW CREATE TABLE statement. Column defaults higher up
// use the same quoting, so only the text after the last PROPERTIES is read.
func parseProperties(ddl string) map[string]string {
	idx := strings.LastIndex(strings.ToUpper(ddl), "PROPERTIES")
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

// parenGroup returns the text inside the parenthesised group whose opening
// bracket is at or after start, honouring nested brackets so expressions
// like date_trunc(`d`, 'day') come back whole.
func parenGroup(s string, start int) (string, bool) {
	open := strings.IndexByte(s[start:], '(')
	if open < 0 {
		return "", false
	}
	open += start

	depth := 0
	for i := open; i < len(s); i++ {
		switch s[i] {
		case '(':
			depth++
		case ')':
			depth--
			if depth == 0 {
				return s[open+1 : i], true
			}
		}
	}
	return "", false
}

// splitIdentifiers splits a comma separated identifier list, ignoring
// commas inside nested brackets, and strips the backticks Doris quotes
// every identifier with.
func splitIdentifiers(group string) []string {
	var parts []string
	depth := 0
	start := 0
	for i := 0; i < len(group); i++ {
		switch group[i] {
		case '(':
			depth++
		case ')':
			depth--
		case ',':
			if depth == 0 {
				parts = append(parts, group[start:i])
				start = i + 1
			}
		}
	}
	parts = append(parts, group[start:])

	var identifiers []string
	for _, p := range parts {
		p = strings.TrimSpace(strings.ReplaceAll(p, "`", ""))
		if p != "" {
			identifiers = append(identifiers, p)
		}
	}
	return identifiers
}

var viewBodyRe = regexp.MustCompile(`(?is)\sAS\s+((?:SELECT|WITH|\().*)$`)

// extractViewQuery returns the SELECT a CREATE VIEW statement defines, so
// the asset's query panel shows the query rather than the wrapper around
// it. The full statement is returned when the body cannot be found.
func extractViewQuery(ddl string) string {
	m := viewBodyRe.FindStringSubmatch(ddl)
	if m == nil {
		return strings.TrimSpace(ddl)
	}
	return strings.TrimSuffix(strings.TrimSpace(m[1]), ";")
}

// tableRef is a table named in a query, split into the parts Doris allows.
// Database is empty for a bare name.
type tableRef struct {
	Database string
	Table    string
}

// referencedTables lists the tables and views a query reads, in order of
// first appearance. It follows every FROM and JOIN, including the comma
// separated form (FROM a, b) and nested subqueries, and skips function
// calls such as FROM mv_infos(...). It is deliberately shallow: aliases and
// CTE names come back as references too and are dropped later when they
// do not resolve to a discovered object.
func referencedTables(query string) []tableRef {
	tokens := tokenize(query)

	var refs []tableRef
	seen := make(map[tableRef]struct{})
	add := func(ref tableRef) {
		if _, ok := seen[ref]; ok {
			return
		}
		seen[ref] = struct{}{}
		refs = append(refs, ref)
	}

	for i := 0; i < len(tokens); i++ {
		if !tokens[i].keyword("FROM", "JOIN") {
			continue
		}

		j := i + 1
		for j < len(tokens) {
			ref, ok := tokens[j].ref()
			if !ok {
				break
			}
			// A name followed by an opening bracket is a function.
			if j+1 < len(tokens) && tokens[j+1].text == "(" {
				break
			}
			add(ref)
			j++

			// Skip an optional alias so FROM a x, b y still finds b.
			if j < len(tokens) && tokens[j].keyword("AS") {
				j++
			}
			if j < len(tokens) && tokens[j].kind == tokenIdent && !tokens[j].reserved() {
				j++
			}
			if j < len(tokens) && tokens[j].text == "," {
				j++
				continue
			}
			break
		}
	}

	return refs
}

type tokenKind int

const (
	tokenIdent tokenKind = iota
	tokenString
	tokenPunct
)

type token struct {
	kind tokenKind
	text string
}

func (t token) keyword(words ...string) bool {
	if t.kind != tokenIdent {
		return false
	}
	for _, w := range words {
		if strings.EqualFold(t.text, w) {
			return true
		}
	}
	return false
}

// reservedWords are the words that can follow a table name in a FROM
// clause without being an alias, and the ones that can never be a table.
var reservedWords = map[string]struct{}{
	"AS": {}, "CROSS": {}, "EXCEPT": {}, "FROM": {}, "FULL": {}, "GROUP": {},
	"HAVING": {}, "INNER": {}, "INTERSECT": {}, "JOIN": {}, "LATERAL": {},
	"LEFT": {}, "LIMIT": {}, "NATURAL": {}, "ON": {}, "ORDER": {}, "OUTER": {},
	"QUALIFY": {}, "RIGHT": {}, "SELECT": {}, "SET": {}, "UNION": {},
	"USING": {}, "VALUES": {}, "WHERE": {}, "WINDOW": {}, "WITH": {},
}

func (t token) reserved() bool {
	_, ok := reservedWords[strings.ToUpper(t.text)]
	return ok
}

// ref reads a token as a table reference: bare, database qualified, or
// catalog qualified (the catalog is dropped).
func (t token) ref() (tableRef, bool) {
	if t.kind != tokenIdent || t.reserved() {
		return tableRef{}, false
	}
	parts := strings.Split(t.text, ".")
	switch len(parts) {
	case 1:
		return tableRef{Table: parts[0]}, true
	case 2:
		return tableRef{Database: parts[0], Table: parts[1]}, true
	case 3:
		return tableRef{Database: parts[1], Table: parts[2]}, true
	default:
		return tableRef{}, false
	}
}

// tokenize splits SQL into identifiers (dotted and backtick quoted names
// collapse into one token with the quotes removed), string literals and
// single-character punctuation. Comments are dropped.
func tokenize(query string) []token {
	var tokens []token
	i := 0
	for i < len(query) {
		c := query[i]
		switch {
		case c == ' ' || c == '\t' || c == '\n' || c == '\r':
			i++
		case c == '-' && i+1 < len(query) && query[i+1] == '-':
			nl := strings.IndexByte(query[i:], '\n')
			if nl < 0 {
				i = len(query)
			} else {
				i += nl
			}
		case c == '/' && i+1 < len(query) && query[i+1] == '*':
			end := strings.Index(query[i:], "*/")
			if end < 0 {
				i = len(query)
			} else {
				i += end + 2
			}
		case c == '\'' || c == '"':
			end := i + 1
			for end < len(query) && query[end] != c {
				if query[end] == '\\' {
					end++
				}
				end++
			}
			tokens = append(tokens, token{kind: tokenString, text: query[i:min(end+1, len(query))]})
			i = end + 1
		case c == '`' || isIdentByte(c):
			var name strings.Builder
			for i < len(query) {
				if query[i] == '`' {
					end := strings.IndexByte(query[i+1:], '`')
					if end < 0 {
						name.WriteString(query[i+1:])
						i = len(query)
						break
					}
					name.WriteString(query[i+1 : i+1+end])
					i += end + 2
				} else if isIdentByte(query[i]) {
					start := i
					for i < len(query) && isIdentByte(query[i]) {
						i++
					}
					name.WriteString(query[start:i])
				} else {
					break
				}
				if i < len(query) && query[i] == '.' {
					name.WriteByte('.')
					i++
					continue
				}
				break
			}
			tokens = append(tokens, token{kind: tokenIdent, text: name.String()})
		default:
			tokens = append(tokens, token{kind: tokenPunct, text: string(c)})
			i++
		}
	}
	return tokens
}

func isIdentByte(c byte) bool {
	return c == '_' || c == '$' ||
		(c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') ||
		c >= 0x80
}

var foreignKeyRe = regexp.MustCompile(`(?i)FOREIGN\s+KEY\s*\(([^)]*)\)\s*REFERENCES\s+([^\s(]+)\s*\(([^)]*)\)`)

// foreignKey is one FOREIGN KEY constraint as SHOW CONSTRAINTS prints it.
type foreignKey struct {
	Columns           []string
	ReferencedTable   tableRef
	ReferencedColumns []string
}

// parseForeignKey reads a SHOW CONSTRAINTS definition such as
// FOREIGN KEY (customer_id) REFERENCES shop.customers (customer_id).
func parseForeignKey(definition string) (foreignKey, bool) {
	m := foreignKeyRe.FindStringSubmatch(definition)
	if m == nil {
		return foreignKey{}, false
	}

	ref, ok := token{kind: tokenIdent, text: strings.ReplaceAll(m[2], "`", "")}.ref()
	if !ok {
		return foreignKey{}, false
	}

	return foreignKey{
		Columns:           splitIdentifiers(m[1]),
		ReferencedTable:   ref,
		ReferencedColumns: splitIdentifiers(m[3]),
	}, true
}
