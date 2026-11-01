package grafana

import (
	"strings"
	"unicode"

	"github.com/marmotdata/plugin-sdk/mrn"
)

// sqlSource describes how the tables read through one Grafana data
// source type are catalogued by the Marmot plugin for that technology.
// A lineage edge only lands when it uses the exact provider and name
// shape that plugin produces, so both are pinned here.
type sqlSource struct {
	// Provider is the Marmot provider string of the technology, used
	// verbatim as the service component of the table's MRN.
	Provider string

	// tableName builds the table's Marmot name from the parts of a
	// reference in the query. database is the reference's own database
	// when it has one, otherwise the data source's configured database.
	tableName func(database, schema, table string) string
}

// datasourceMap maps the Grafana data source types that run SQL to the
// naming rule of the native Marmot plugin for the same technology.
// Older Grafana releases report the PostgreSQL data source as postgres.
var datasourceMap = map[string]sqlSource{
	"grafana-postgresql-datasource":    {Provider: "PostgreSQL", tableName: bareName},
	"postgres":                         {Provider: "PostgreSQL", tableName: bareName},
	"mysql":                            {Provider: "MySQL", tableName: bareName},
	"mssql":                            {Provider: "SQL Server", tableName: sqlServerName},
	"grafana-clickhouse-datasource":    {Provider: "ClickHouse", tableName: bareName},
	"vertamedia-clickhouse-datasource": {Provider: "ClickHouse", tableName: bareName},
}

// bareName names a table by its own name, the way the PostgreSQL,
// MySQL and ClickHouse plugins do.
func bareName(_, _, table string) string {
	return table
}

// sqlServerName names a table database.schema.table with the schema
// defaulting to dbo, the way the SQL Server plugin does. Without a
// database there is no name that plugin would recognise, so none is
// produced.
func sqlServerName(database, schema, table string) string {
	if database == "" {
		return ""
	}
	if schema == "" {
		schema = "dbo"
	}
	return database + "." + schema + "." + table
}

// tableRef is one table a query reads, split into the parts the query
// qualified it with.
type tableRef struct {
	Database string
	Schema   string
	Table    string
}

// extractTables returns the tables a SQL query reads: every name after
// FROM or JOIN that is a plain table reference. Subqueries, function
// calls, CTE names, Grafana macros and template variables produce
// nothing, on the principle that a missing edge is better than one to
// a table that does not exist.
func extractTables(sql string) []tableRef {
	toks := tokenize(sql)
	ctes := cteNames(toks)

	var refs []tableRef
	seen := make(map[tableRef]bool)
	add := func(ref tableRef) {
		if ref.Schema == "" && ref.Database == "" && ctes[strings.ToLower(ref.Table)] {
			return
		}
		if !seen[ref] {
			seen[ref] = true
			refs = append(refs, ref)
		}
	}

	for i, t := range toks {
		if !t.isKeyword("from") && !t.isKeyword("join") {
			continue
		}
		// ClickHouse's ARRAY JOIN unrolls a column, not a table.
		if t.isKeyword("join") && i > 0 && toks[i-1].isKeyword("array") {
			continue
		}

		next := i + 1
		for {
			ref, after, ok := parseTableRef(toks, next)
			if ok {
				add(ref)
			}
			if !t.isKeyword("from") {
				break
			}
			// A FROM list may name several tables separated by commas,
			// each with an optional alias.
			after = skipAlias(toks, after)
			if after >= len(toks) || !toks[after].isPunct(",") {
				break
			}
			next = after + 1
		}
	}

	return refs
}

// parseTableRef reads a possibly qualified table name starting at
// toks[i]. It reports false for anything that is not a table: a
// subquery, a function call, a template variable, or a keyword that
// happens to follow FROM.
func parseTableRef(toks []token, i int) (tableRef, int, bool) {
	if i >= len(toks) || toks[i].kind != tokIdent {
		return tableRef{}, i, false
	}

	parts := []string{toks[i].text}
	next := i + 1
	for next+1 < len(toks) && toks[next].isPunct(".") {
		switch toks[next+1].kind {
		case tokIdent:
			parts = append(parts, toks[next+1].text)
			next += 2
			continue
		default:
			// schema.$table: a variable inside the name.
			return tableRef{}, next, false
		}
	}

	if next < len(toks) && toks[next].isPunct("(") {
		return tableRef{}, next, false
	}
	if len(parts) == 1 && !toks[i].quoted && nonTableWords[strings.ToLower(parts[0])] {
		return tableRef{}, next, false
	}

	// Four-part SQL Server names carry the server first; the last
	// three parts are what identifies the table.
	if len(parts) > 3 {
		parts = parts[len(parts)-3:]
	}

	ref := tableRef{Table: parts[len(parts)-1]}
	if len(parts) >= 2 {
		ref.Schema = parts[len(parts)-2]
	}
	if len(parts) == 3 {
		ref.Database = parts[0]
	}
	return ref, next, true
}

// skipAlias advances past an optional table alias so a following comma
// can be seen.
func skipAlias(toks []token, i int) int {
	if i < len(toks) && toks[i].isKeyword("as") {
		i++
	}
	if i < len(toks) && toks[i].kind == tokIdent && (toks[i].quoted || !clauseWords[strings.ToLower(toks[i].text)]) {
		i++
	}
	return i
}

// cteNames returns the names defined in a WITH clause, lower-cased.
// Each is an identifier preceded by WITH, RECURSIVE or a comma and
// followed by an optional column list, AS and an opening parenthesis.
func cteNames(toks []token) map[string]bool {
	names := make(map[string]bool)
	for i, t := range toks {
		if t.kind != tokIdent || i == 0 {
			continue
		}
		prev := toks[i-1]
		if !prev.isKeyword("with") && !prev.isKeyword("recursive") && !prev.isPunct(",") {
			continue
		}

		next := i + 1
		if next < len(toks) && toks[next].isPunct("(") {
			next = closingParen(toks, next) + 1
		}
		if next+1 < len(toks) && toks[next].isKeyword("as") && toks[next+1].isPunct("(") {
			names[strings.ToLower(t.text)] = true
		}
	}
	return names
}

// closingParen returns the index of the parenthesis closing the one at
// toks[open], or the end of the tokens when it is unbalanced.
func closingParen(toks []token, open int) int {
	depth := 0
	for i := open; i < len(toks); i++ {
		switch {
		case toks[i].isPunct("("):
			depth++
		case toks[i].isPunct(")"):
			depth--
			if depth == 0 {
				return i
			}
		}
	}
	return len(toks)
}

// nonTableWords are keywords that can directly follow FROM or JOIN in
// place of a table name.
var nonTableWords = map[string]bool{
	"lateral": true, "only": true, "unnest": true, "values": true, "select": true, "dual": true,
}

// clauseWords are the keywords that can follow a table reference, so
// they are never read as its alias.
var clauseWords = map[string]bool{
	"where": true, "on": true, "join": true, "left": true, "right": true, "inner": true,
	"outer": true, "full": true, "cross": true, "natural": true, "group": true, "order": true,
	"limit": true, "union": true, "except": true, "intersect": true, "having": true,
	"window": true, "using": true, "set": true, "fetch": true, "offset": true, "for": true,
	"with": true, "select": true, "and": true, "or": true, "not": true, "lateral": true,
	"final": true, "prewhere": true, "sample": true, "settings": true, "format": true,
	"array": true, "tablesample": true, "straight_join": true, "into": true, "returning": true,
	"qualify": true, "when": true, "then": true, "else": true, "end": true, "case": true,
	"in": true, "is": true, "like": true, "between": true, "exists": true, "values": true,
}

type tokenKind int

const (
	tokIdent tokenKind = iota
	tokPunct
	tokVar
)

// token is one lexical unit of a query. Identifiers keep their text
// with any quoting removed; punctuation keeps the character; template
// variables and macros are marked so they can never become a name.
type token struct {
	kind   tokenKind
	text   string
	quoted bool
}

func (t token) isKeyword(word string) bool {
	return t.kind == tokIdent && !t.quoted && strings.EqualFold(t.text, word)
}

func (t token) isPunct(p string) bool {
	return t.kind == tokPunct && t.text == p
}

// tokenize splits a query into tokens. Comments and string literals
// are consumed so a FROM inside either cannot be mistaken for the real
// one. Grafana's $__macros, $var, ${var} and [[var]] forms all become
// variable tokens.
func tokenize(sql string) []token {
	var toks []token
	runes := []rune(sql)
	n := len(runes)

	for i := 0; i < n; {
		r := runes[i]

		switch {
		case unicode.IsSpace(r):
			i++

		case r == '-' && i+1 < n && runes[i+1] == '-':
			for i < n && runes[i] != '\n' {
				i++
			}

		case r == '/' && i+1 < n && runes[i+1] == '*':
			end := strings.Index(string(runes[i+2:]), "*/")
			if end < 0 {
				i = n
			} else {
				i += 2 + len([]rune(string(runes[i+2:])[:end])) + 2
			}

		case r == '\'':
			// A string literal; '' inside it is an escaped quote.
			i++
			for i < n {
				if runes[i] == '\'' {
					if i+1 < n && runes[i+1] == '\'' {
						i += 2
						continue
					}
					break
				}
				i++
			}
			i++
			toks = append(toks, token{kind: tokPunct, text: "'"})

		case r == '"' || r == '`':
			text, next := readQuoted(runes, i+1, r)
			toks = append(toks, token{kind: tokIdent, text: text, quoted: true})
			i = next

		case r == '[':
			if i+1 < n && runes[i+1] == '[' {
				// Grafana's old [[var]] template syntax.
				end := strings.Index(string(runes[i:]), "]]")
				if end < 0 {
					i = n
				} else {
					i += len([]rune(string(runes[i:])[:end])) + 2
				}
				toks = append(toks, token{kind: tokVar})
				continue
			}
			text, next := readQuoted(runes, i+1, ']')
			toks = append(toks, token{kind: tokIdent, text: text, quoted: true})
			i = next

		case r == '$':
			i++
			if i < n && runes[i] == '{' {
				for i < n && runes[i] != '}' {
					i++
				}
				i++
			} else {
				for i < n && isWordRune(runes[i]) {
					i++
				}
			}
			toks = append(toks, token{kind: tokVar})

		case isWordRune(r):
			start := i
			for i < n && isWordRune(runes[i]) {
				i++
			}
			toks = append(toks, token{kind: tokIdent, text: string(runes[start:i])})

		default:
			toks = append(toks, token{kind: tokPunct, text: string(r)})
			i++
		}
	}

	return toks
}

// readQuoted reads a quoted identifier whose opening quote is at
// runes[start-1] and returns its text and the index after the closing
// quote. A doubled closing quote is an escaped one.
func readQuoted(runes []rune, start int, closing rune) (string, int) {
	var b strings.Builder
	i := start
	for i < len(runes) {
		if runes[i] == closing {
			if i+1 < len(runes) && runes[i+1] == closing {
				b.WriteRune(closing)
				i += 2
				continue
			}
			return b.String(), i + 1
		}
		b.WriteRune(runes[i])
		i++
	}
	return b.String(), i
}

func isWordRune(r rune) bool {
	return r == '_' || unicode.IsLetter(r) || unicode.IsDigit(r)
}

// sqlTableMRNs returns the MRNs of the tables a query reads through a
// data source of the given type, named the way the native plugin for
// that technology names them. Data source types that do not run SQL
// yield nothing.
func sqlTableMRNs(datasourceType, database, sql string) []string {
	src, ok := datasourceMap[datasourceType]
	if !ok || strings.TrimSpace(sql) == "" {
		return nil
	}

	var out []string
	for _, ref := range extractTables(sql) {
		db := ref.Database
		if db == "" {
			db = database
		}
		name := src.tableName(db, ref.Schema, ref.Table)
		if name == "" {
			continue
		}
		out = append(out, mrn.New("Table", src.Provider, name))
	}
	return out
}
