package redash

import "strings"

// tableRef is one table named in a query, split on dots. One part is a bare
// name, two are schema and table, three are database, schema and table.
type tableRef struct {
	Parts []string
}

// Table is the object's own name.
func (r tableRef) Table() string { return r.Parts[len(r.Parts)-1] }

// Schema is the qualifier directly above the table, empty when the reference
// was bare.
func (r tableRef) Schema() string {
	if len(r.Parts) < 2 {
		return ""
	}
	return r.Parts[len(r.Parts)-2]
}

// Database is the top qualifier, empty unless the reference had three parts.
func (r tableRef) Database() string {
	if len(r.Parts) < 3 {
		return ""
	}
	return r.Parts[len(r.Parts)-3]
}

// String rejoins the parts, used for logging and test assertions.
func (r tableRef) String() string { return strings.Join(r.Parts, ".") }

// fromArgFunctions are the functions that take FROM as an argument separator
// rather than as a clause, for example extract(year from ts). Without this
// the column after FROM would be read as a table.
var fromArgFunctions = map[string]bool{
	"extract":   true,
	"substring": true,
	"position":  true,
	"overlay":   true,
	"trim":      true,
}

// notTableWords are words that can follow FROM or JOIN in place of a table
// name. What comes after them is a subquery or a function, never a table.
var notTableWords = map[string]bool{
	"select":  true,
	"lateral": true,
	"unnest":  true,
	"values":  true,
	"table":   true,
}

// skipWords sit between FROM and the table name without being part of it.
var skipWords = map[string]bool{
	"only": true,
}

// extractTableRefs finds the tables a query reads. It scans for FROM and JOIN
// clauses rather than parsing SQL, which is enough to catalogue lineage and
// stays dialect-neutral: Redash runs one query text against whichever engine
// the data source points at.
//
// It skips derived tables, function calls, common table expressions and the
// columns of functions that spell an argument with FROM.
func extractTableRefs(sql string) []tableRef {
	tokens := tokenize(stripPlaceholders(sql))
	ctes := cteNames(tokens)

	var refs []tableRef
	seen := make(map[string]struct{})

	// funcStack records the function name owning each open parenthesis so a
	// FROM inside extract(...) can be told from a FROM opening a clause.
	var funcStack []string

	for i := 0; i < len(tokens); i++ {
		tok := tokens[i]

		if tok.kind == tokenPunct {
			switch tok.text {
			case "(":
				funcStack = append(funcStack, precedingWord(tokens, i))
			case ")":
				if len(funcStack) > 0 {
					funcStack = funcStack[:len(funcStack)-1]
				}
			}
			continue
		}

		if tok.kind != tokenWord {
			continue
		}

		keyword := strings.ToLower(tok.text)
		if keyword != "from" && keyword != "join" {
			continue
		}
		if keyword == "from" && len(funcStack) > 0 && fromArgFunctions[funcStack[len(funcStack)-1]] {
			continue
		}

		ref, next, ok := readTableRef(tokens, i+1)
		i = next - 1
		if !ok {
			continue
		}
		if len(ref.Parts) == 1 && ctes[strings.ToLower(ref.Parts[0])] {
			continue
		}

		key := strings.ToLower(ref.String())
		if _, dup := seen[key]; dup {
			continue
		}
		seen[key] = struct{}{}
		refs = append(refs, ref)
	}

	return refs
}

// readTableRef reads a dotted table reference starting at index start. It
// returns the reference, the index just past it, and whether one was found.
func readTableRef(tokens []token, start int) (tableRef, int, bool) {
	for start < len(tokens) && tokens[start].kind == tokenWord && skipWords[strings.ToLower(tokens[start].text)] {
		start++
	}
	if start >= len(tokens) {
		return tableRef{}, start, false
	}

	first := tokens[start]
	if first.kind == tokenWord && notTableWords[strings.ToLower(first.text)] {
		return tableRef{}, start + 1, false
	}
	if first.kind != tokenWord && first.kind != tokenQuoted {
		// A parenthesis here opens a derived table, anything else is not a
		// name.
		return tableRef{}, start + 1, false
	}

	parts := []string{first.text}
	i := start + 1
	for i+1 < len(tokens) && tokens[i].kind == tokenPunct && tokens[i].text == "." {
		next := tokens[i+1]
		if next.kind != tokenWord && next.kind != tokenQuoted {
			break
		}
		parts = append(parts, next.text)
		i += 2
	}

	// A name followed directly by "(" is a function call, not a table.
	if i < len(tokens) && tokens[i].kind == tokenPunct && tokens[i].text == "(" {
		return tableRef{}, i, false
	}

	return tableRef{Parts: parts}, i, true
}

// cteNames collects the names bound by WITH clauses. A common table
// expression is the only thing written as "name AS (", so matching that shape
// finds them without tracking WITH nesting. Column aliases are "AS name" with
// no parenthesis and derived table aliases come after ")".
func cteNames(tokens []token) map[string]bool {
	names := make(map[string]bool)

	for i := 0; i+2 < len(tokens); i++ {
		name := tokens[i]
		if name.kind != tokenWord && name.kind != tokenQuoted {
			continue
		}
		if tokens[i+1].kind != tokenWord || !strings.EqualFold(tokens[i+1].text, "as") {
			continue
		}
		if tokens[i+2].kind != tokenPunct || tokens[i+2].text != "(" {
			continue
		}
		names[strings.ToLower(name.text)] = true
	}

	return names
}

// precedingWord returns the lowercased word immediately before index i, used
// to name the function that opened a parenthesis.
func precedingWord(tokens []token, i int) string {
	if i == 0 || tokens[i-1].kind != tokenWord {
		return ""
	}
	return strings.ToLower(tokens[i-1].text)
}

// stripPlaceholders blanks out Redash's template syntax. A {{param}} stands
// where a literal goes and {% if %} wraps optional SQL; neither is a table,
// and leaving the braces in would confuse the scanner.
func stripPlaceholders(sql string) string {
	var b strings.Builder
	b.Grow(len(sql))

	for i := 0; i < len(sql); i++ {
		if i+1 < len(sql) && sql[i] == '{' && (sql[i+1] == '{' || sql[i+1] == '%') {
			closing := "}}"
			if sql[i+1] == '%' {
				closing = "%}"
			}
			end := strings.Index(sql[i:], closing)
			if end >= 0 {
				b.WriteByte(' ')
				i += end + 1
				continue
			}
		}
		b.WriteByte(sql[i])
	}

	return b.String()
}

type tokenKind int

const (
	// tokenWord is a bare identifier or keyword.
	tokenWord tokenKind = iota
	// tokenQuoted is a quoted identifier, with the quotes removed.
	tokenQuoted
	// tokenPunct is a single punctuation character.
	tokenPunct
	// tokenString is a string literal, kept so its contents are never read
	// as SQL.
	tokenString
)

type token struct {
	kind tokenKind
	text string
}

// tokenize splits SQL into the pieces the reference scanner needs. It is
// deliberately dialect-neutral: it understands the quoting styles the engines
// Redash connects to use, and treats everything else as punctuation.
func tokenize(sql string) []token {
	var tokens []token

	for i := 0; i < len(sql); {
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

		case c == '\'':
			text, next := readDelimited(sql, i, '\'', '\'')
			tokens = append(tokens, token{kind: tokenString, text: text})
			i = next

		case c == '"':
			text, next := readDelimited(sql, i, '"', '"')
			tokens = append(tokens, token{kind: tokenQuoted, text: text})
			i = next

		case c == '`':
			text, next := readDelimited(sql, i, '`', '`')
			tokens = append(tokens, token{kind: tokenQuoted, text: text})
			i = next

		case c == '[':
			text, next := readDelimited(sql, i, '[', ']')
			tokens = append(tokens, token{kind: tokenQuoted, text: text})
			i = next

		case isWordByte(c):
			start := i
			for i < len(sql) && isWordByte(sql[i]) {
				i++
			}
			tokens = append(tokens, token{kind: tokenWord, text: sql[start:i]})

		default:
			tokens = append(tokens, token{kind: tokenPunct, text: string(c)})
			i++
		}
	}

	return tokens
}

// readDelimited reads a quoted run starting at the opening delimiter and
// returns its contents plus the index just past the close. A doubled closing
// delimiter escapes itself, which is how every engine here escapes quotes.
func readDelimited(sql string, start int, open, close byte) (string, int) {
	var b strings.Builder
	i := start + 1

	for i < len(sql) {
		if sql[i] == close {
			if i+1 < len(sql) && sql[i+1] == close && open == close {
				b.WriteByte(close)
				i += 2
				continue
			}
			return b.String(), i + 1
		}
		b.WriteByte(sql[i])
		i++
	}

	return b.String(), i
}

func isWordByte(c byte) bool {
	return c == '_' || c == '$' ||
		(c >= 'a' && c <= 'z') ||
		(c >= 'A' && c <= 'Z') ||
		(c >= '0' && c <= '9') ||
		c >= 0x80
}
