package oracle

import (
	"fmt"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

// normaliseSchemaNames folds configured schema names to the form Oracle
// stores them in: upper case unless the name was written in double quotes,
// which keeps its case. Empty entries and duplicates are dropped.
func normaliseSchemaNames(names []string) []string {
	var result []string
	seen := make(map[string]struct{}, len(names))
	for _, name := range names {
		name = strings.TrimSpace(name)
		if len(name) >= 2 && strings.HasPrefix(name, `"`) && strings.HasSuffix(name, `"`) {
			name = name[1 : len(name)-1]
		} else {
			name = strings.ToUpper(name)
		}
		if name == "" {
			continue
		}
		if _, dup := seen[name]; dup {
			continue
		}
		seen[name] = struct{}{}
		result = append(result, name)
	}
	return result
}

// renderDataType writes a column's type the way DESCRIBE shows it, so
// NUMBER(10,2) and VARCHAR2(50 CHAR) read the same in Marmot as in Oracle.
func renderDataType(c columnRow) string {
	switch c.DataType {
	case "NUMBER":
		switch {
		case c.Precision.Valid && c.Scale.Valid && c.Scale.Int64 != 0:
			return fmt.Sprintf("NUMBER(%d,%d)", c.Precision.Int64, c.Scale.Int64)
		case c.Precision.Valid:
			return fmt.Sprintf("NUMBER(%d)", c.Precision.Int64)
		case c.Scale.Valid && c.Scale.Int64 == 0:
			// INTEGER and NUMBER(*,0) are stored with no precision and a
			// zero scale; DESCRIBE reports them as NUMBER(38).
			return "NUMBER(38)"
		case c.Scale.Valid:
			return fmt.Sprintf("NUMBER(*,%d)", c.Scale.Int64)
		default:
			return "NUMBER"
		}
	case "FLOAT":
		if c.Precision.Valid {
			return fmt.Sprintf("FLOAT(%d)", c.Precision.Int64)
		}
		return "FLOAT"
	case "VARCHAR2", "VARCHAR", "CHAR":
		if !c.CharLength.Valid {
			return c.DataType
		}
		if c.CharUsed.String == "C" {
			return fmt.Sprintf("%s(%d CHAR)", c.DataType, c.CharLength.Int64)
		}
		return fmt.Sprintf("%s(%d)", c.DataType, c.CharLength.Int64)
	case "NVARCHAR2", "NCHAR":
		// National character types always measure in characters.
		if c.CharLength.Valid {
			return fmt.Sprintf("%s(%d)", c.DataType, c.CharLength.Int64)
		}
		return c.DataType
	case "RAW":
		if c.DataLength.Valid {
			return fmt.Sprintf("RAW(%d)", c.DataLength.Int64)
		}
		return c.DataType
	default:
		// DATE, CLOB, TIMESTAMP(6) WITH TIME ZONE, INTERVAL DAY(2) TO
		// SECOND(6) and friends already carry their parameters in the name.
		return c.DataType
	}
}

// objectRef identifies a table or view by schema and name, in the case
// Oracle stores them.
type objectRef struct {
	Owner string
	Name  string
}

type foreignKey struct {
	Constraint  string
	SourceOwner string
	SourceTable string
	TargetOwner string
	TargetTable string
	DeleteRule  string
}

// primaryKeyColumns returns, per table, the set of columns in its primary
// key. rows are expected to come from one schema.
func primaryKeyColumns(rows []constraintRow) map[string]map[string]bool {
	keys := make(map[string]map[string]bool)
	for _, r := range rows {
		if r.Type != "P" {
			continue
		}
		if keys[r.Table] == nil {
			keys[r.Table] = make(map[string]bool)
		}
		keys[r.Table][r.Column] = true
	}
	return keys
}

// resolveForeignKeys pairs every foreign key with the table it references.
// Oracle records the target of a foreign key as another constraint (the
// primary or unique key it points at), not as a table, so the referenced
// table is read off that constraint's own row. A reference to a constraint
// absent from rows, because its schema was not read, is dropped. Constraints
// span several rows when they cover several columns; one key is returned
// per constraint.
func resolveForeignKeys(rows []constraintRow) []foreignKey {
	type constraintKey struct{ owner, name string }
	tableByConstraint := make(map[constraintKey]string)
	for _, r := range rows {
		if r.Type == "P" || r.Type == "U" {
			tableByConstraint[constraintKey{r.Owner, r.Name}] = r.Table
		}
	}

	var keys []foreignKey
	seen := make(map[constraintKey]struct{})
	for _, r := range rows {
		if r.Type != "R" || !r.ROwner.Valid || !r.RConstraint.Valid {
			continue
		}
		id := constraintKey{r.Owner, r.Name}
		if _, dup := seen[id]; dup {
			continue
		}
		seen[id] = struct{}{}

		target, ok := tableByConstraint[constraintKey{r.ROwner.String, r.RConstraint.String}]
		if !ok {
			continue
		}
		keys = append(keys, foreignKey{
			Constraint:  r.Name,
			SourceOwner: r.Owner,
			SourceTable: r.Table,
			TargetOwner: r.ROwner.String,
			TargetTable: target,
			DeleteRule:  r.DeleteRule.String,
		})
	}
	return keys
}

// A token from view SQL: an identifier, keyword or single punctuation
// character. Quoted identifiers keep their case; everything else is compared
// case-insensitively.
type token struct {
	text   string
	quoted bool
}

// tokenize splits SQL into tokens, dropping comments and string literals,
// which is all the reference extraction needs.
func tokenize(sql string) []token {
	var tokens []token
	i := 0
	for i < len(sql) {
		c := sql[i]
		switch {
		case c == ' ' || c == '\t' || c == '\n' || c == '\r':
			i++
		case c == '-' && i+1 < len(sql) && sql[i+1] == '-':
			for i < len(sql) && sql[i] != '\n' {
				i++
			}
		case c == '/' && i+1 < len(sql) && sql[i+1] == '*':
			end := strings.Index(sql[i+2:], "*/")
			if end < 0 {
				i = len(sql)
			} else {
				i += end + 4
			}
		case c == '"':
			var b strings.Builder
			j := i + 1
			for j < len(sql) {
				if sql[j] == '"' {
					if j+1 < len(sql) && sql[j+1] == '"' {
						b.WriteByte('"')
						j += 2
						continue
					}
					break
				}
				b.WriteByte(sql[j])
				j++
			}
			tokens = append(tokens, token{text: b.String(), quoted: true})
			i = j + 1
		case c == '\'':
			j := i + 1
			for j < len(sql) {
				if sql[j] == '\'' {
					if j+1 < len(sql) && sql[j+1] == '\'' {
						j += 2
						continue
					}
					break
				}
				j++
			}
			i = j + 1
		case isIdentifierByte(c):
			j := i
			for j < len(sql) && isIdentifierByte(sql[j]) {
				j++
			}
			tokens = append(tokens, token{text: sql[i:j]})
			i = j
		default:
			tokens = append(tokens, token{text: string(c)})
			i++
		}
	}
	return tokens
}

func isIdentifierByte(c byte) bool {
	return c == '_' || c == '$' || c == '#' ||
		(c >= '0' && c <= '9') || (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || c >= 0x80
}

// reservedAfterFrom are the words that can follow FROM or JOIN, or a table
// name, without being a table name themselves.
var reservedAfterFrom = map[string]bool{
	"SELECT": true, "WHERE": true, "ON": true, "AS": true, "JOIN": true, "INNER": true,
	"LEFT": true, "RIGHT": true, "FULL": true, "CROSS": true, "NATURAL": true, "OUTER": true,
	"GROUP": true, "ORDER": true, "UNION": true, "MINUS": true, "INTERSECT": true, "HAVING": true,
	"START": true, "CONNECT": true, "MODEL": true, "PIVOT": true, "UNPIVOT": true, "FETCH": true,
	"FOR": true, "WITH": true, "PARTITION": true, "USING": true, "LATERAL": true, "TABLE": true,
	"ONLY": true, "SAMPLE": true, "VERSIONS": true, "OFFSET": true, "LIMIT": true, "WINDOW": true,
}

// extractViewReferences lists the objects a view's SQL reads from: every
// name after FROM or JOIN, including comma-separated lists. Bare names are
// upper-cased the way Oracle stores them; quoted names keep their case. An
// unqualified name has an empty Owner. Common table expressions, subqueries
// and objects reached over a database link are not references to a local
// object and are left out.
func extractViewReferences(text string) []objectRef {
	tokens := tokenize(text)
	ctes := commonTableExpressions(tokens)

	var refs []objectRef
	seen := make(map[objectRef]struct{})
	add := func(ref objectRef) {
		if ref.Owner == "" && ctes[ref.Name] {
			return
		}
		if _, dup := seen[ref]; dup {
			return
		}
		seen[ref] = struct{}{}
		refs = append(refs, ref)
	}

	for i := 0; i < len(tokens); i++ {
		if tokens[i].quoted {
			continue
		}
		keyword := strings.ToUpper(tokens[i].text)
		if keyword != "FROM" && keyword != "JOIN" {
			continue
		}
		j := i + 1
		for j < len(tokens) {
			ref, next, ok := parseObjectRef(tokens, j)
			if !ok {
				break
			}
			j = next
			if j < len(tokens) && tokens[j].text == "@" {
				// A database link: the object lives in another database.
				_, j, _ = parseObjectRef(tokens, j+1)
			} else {
				add(ref)
			}
			j = skipAlias(tokens, j)
			if keyword == "FROM" && j < len(tokens) && tokens[j].text == "," {
				j++
				continue
			}
			break
		}
	}
	return refs
}

// commonTableExpressions collects the names defined by WITH name AS (...)
// so they are not mistaken for tables when the query reads from them.
func commonTableExpressions(tokens []token) map[string]bool {
	names := make(map[string]bool)
	for i := 0; i+3 < len(tokens); i++ {
		if tokens[i].quoted {
			continue
		}
		lead := strings.ToUpper(tokens[i].text)
		if lead != "WITH" && lead != "," {
			continue
		}
		name, ok := identifierAt(tokens, i+1)
		if !ok {
			continue
		}
		if tokens[i+2].quoted || !strings.EqualFold(tokens[i+2].text, "AS") || tokens[i+3].text != "(" {
			continue
		}
		names[name] = true
	}
	return names
}

// parseObjectRef reads NAME or OWNER.NAME starting at tokens[j] and returns
// the index just past it.
func parseObjectRef(tokens []token, j int) (objectRef, int, bool) {
	first, ok := identifierAt(tokens, j)
	if !ok {
		return objectRef{}, j, false
	}
	j++
	if j+1 < len(tokens) && tokens[j].text == "." && !tokens[j].quoted {
		if second, ok := identifierAt(tokens, j+1); ok {
			return objectRef{Owner: first, Name: second}, j + 2, true
		}
	}
	return objectRef{Name: first}, j, true
}

// identifierAt returns the normalised identifier at tokens[j], or false when
// the token is punctuation or a reserved word.
func identifierAt(tokens []token, j int) (string, bool) {
	if j >= len(tokens) {
		return "", false
	}
	t := tokens[j]
	if t.quoted {
		return t.text, true
	}
	if t.text == "" || !isIdentifierByte(t.text[0]) || (t.text[0] >= '0' && t.text[0] <= '9') {
		return "", false
	}
	upper := strings.ToUpper(t.text)
	if reservedAfterFrom[upper] {
		return "", false
	}
	return upper, true
}

// skipAlias steps over an optional table alias (with or without AS).
func skipAlias(tokens []token, j int) int {
	if j < len(tokens) && !tokens[j].quoted && strings.EqualFold(tokens[j].text, "AS") {
		j++
	}
	if _, ok := identifierAt(tokens, j); ok {
		j++
	}
	return j
}

// quoteIdentifier double-quotes an Oracle identifier so a name is used
// exactly as stored, without the parser upper-casing it.
func quoteIdentifier(name string) string {
	name = strings.ReplaceAll(name, "\x00", "")
	return `"` + strings.ReplaceAll(name, `"`, `""`) + `"`
}

// convertValue turns what the driver returns for a column into a value that
// serialises cleanly as JSON. go-ora hands NUMBER columns over as strings to
// preserve their precision; those are turned back into numbers when they fit.
func convertValue(val any, databaseType string) any {
	switch v := val.(type) {
	case nil:
		return nil
	case string:
		if databaseType == "NUMBER" || databaseType == "FLOAT" || databaseType == "BINARY_FLOAT" || databaseType == "BINARY_DOUBLE" {
			return parseNumber(v)
		}
		return v
	case []byte:
		if utf8.Valid(v) {
			return string(v)
		}
		return fmt.Sprintf("0x%x", v)
	case time.Time:
		return v.Format(time.RFC3339Nano)
	case fmt.Stringer:
		return v.String()
	default:
		return val
	}
}

func parseNumber(s string) any {
	if i, err := strconv.ParseInt(s, 10, 64); err == nil {
		return i
	}
	if f, err := strconv.ParseFloat(s, 64); err == nil {
		return f
	}
	return s
}
