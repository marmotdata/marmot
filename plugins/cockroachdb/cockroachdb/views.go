package cockroachdb

import "strings"

// viewReferences returns the names a view definition reads from: every
// identifier chain that follows FROM or JOIN, and the further ones a comma
// adds to a FROM list. Subqueries and function calls in those positions are
// skipped, as is anything after an alias. Each name is returned as its
// dot-separated parts, unquoted and in the case the engine stores them.
//
// CockroachDB stores view definitions with fully qualified names, so the
// common case is a three-part name; a bare or schema-qualified one still
// comes back for the caller to resolve.
func viewReferences(definition string) [][]string {
	tokens := tokenize(definition)

	var refs [][]string
	seen := make(map[string]struct{})

	for i := 0; i < len(tokens); i++ {
		if !tokens[i].isKeyword("from") && !tokens[i].isKeyword("join") {
			continue
		}

		next := i + 1
		for next < len(tokens) {
			if tokens[next].isKeyword("lateral") || tokens[next].isKeyword("only") {
				next++
				continue
			}

			name, after := readName(tokens, next)
			if name == nil {
				break
			}
			// A name followed by an open paren is a function call, not a table.
			if after < len(tokens) && tokens[after].isPunct("(") {
				break
			}

			joined := strings.Join(name, ".")
			if _, dup := seen[joined]; !dup {
				seen[joined] = struct{}{}
				refs = append(refs, name)
			}

			after = skipAlias(tokens, after)
			if after < len(tokens) && tokens[after].isPunct(",") {
				next = after + 1
				continue
			}
			break
		}
	}

	return refs
}

// resolveReference maps a name from a view definition onto a relation
// discovered in the same database, the way the engine would resolve it: a
// three-part name must be in this database, a two-part name is schema.table
// (or database.table in public), and a bare name is looked up in the view's
// own schema first, then in public.
func resolveReference(parts []string, database, viewSchema string, known map[string]relation) (relation, bool) {
	switch len(parts) {
	case 3:
		if parts[0] != database {
			return relation{}, false
		}
		r, ok := known[parts[1]+"."+parts[2]]
		return r, ok
	case 2:
		if r, ok := known[parts[0]+"."+parts[1]]; ok {
			return r, true
		}
		if parts[0] == database {
			r, ok := known["public."+parts[1]]
			return r, ok
		}
		return relation{}, false
	case 1:
		if r, ok := known[viewSchema+"."+parts[0]]; ok {
			return r, true
		}
		r, ok := known["public."+parts[0]]
		return r, ok
	}
	return relation{}, false
}

type tokenKind int

const (
	tokenIdent   tokenKind = iota // unquoted word: identifier or keyword
	tokenQuoted                   // "quoted identifier", quotes removed
	tokenLiteral                  // 'string literal', contents dropped
	tokenPunct                    // one character of punctuation
)

type token struct {
	kind tokenKind
	text string
}

func (t token) isKeyword(word string) bool {
	return t.kind == tokenIdent && t.text == word
}

func (t token) isPunct(p string) bool {
	return t.kind == tokenPunct && t.text == p
}

func (t token) isName() bool {
	return t.kind == tokenQuoted || (t.kind == tokenIdent && !reservedWords[t.text])
}

// reservedWords are the words that can follow a table name in a FROM
// clause and so cannot be an alias.
var reservedWords = map[string]bool{
	"as": true, "on": true, "using": true, "where": true, "group": true, "order": true,
	"having": true, "limit": true, "offset": true, "fetch": true, "for": true, "union": true,
	"except": true, "intersect": true, "join": true, "inner": true, "left": true, "right": true,
	"full": true, "cross": true, "natural": true, "outer": true, "window": true, "with": true,
	"select": true, "from": true, "lateral": true, "only": true, "values": true, "returning": true,
	"tablesample": true, "and": true, "or": true, "not": true,
}

// readName reads a dot-separated identifier chain starting at tokens[i].
// It returns nil when there is no name there, for example when a subquery
// opens instead.
func readName(tokens []token, i int) ([]string, int) {
	if i >= len(tokens) || !tokens[i].isName() {
		return nil, i
	}
	parts := []string{tokens[i].text}
	i++
	for i+1 < len(tokens) && tokens[i].isPunct(".") && tokens[i+1].isName() {
		parts = append(parts, tokens[i+1].text)
		i += 2
	}
	return parts, i
}

// skipAlias steps over "AS alias" or a bare alias after a table name.
func skipAlias(tokens []token, i int) int {
	if i < len(tokens) && tokens[i].isKeyword("as") {
		i++
		if i < len(tokens) && (tokens[i].kind == tokenIdent || tokens[i].kind == tokenQuoted) {
			i++
		}
		return i
	}
	if i < len(tokens) && tokens[i].isName() {
		i++
	}
	return i
}

// tokenize splits SQL into the few token kinds the reference scan needs.
// Unquoted words are lowercased, which is also how CockroachDB stores
// unquoted identifiers; quoted identifiers keep their case.
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
				return tokens
			}
			i += 2 + end + 2
		case c == '"':
			text, next := readQuoted(sql, i, '"')
			tokens = append(tokens, token{kind: tokenQuoted, text: text})
			i = next
		case c == '\'':
			_, next := readQuoted(sql, i, '\'')
			tokens = append(tokens, token{kind: tokenLiteral})
			i = next
		case isIdentStart(c):
			start := i
			for i < len(sql) && isIdentChar(sql[i]) {
				i++
			}
			tokens = append(tokens, token{kind: tokenIdent, text: strings.ToLower(sql[start:i])})
		default:
			tokens = append(tokens, token{kind: tokenPunct, text: string(c)})
			i++
		}
	}

	return tokens
}

// readQuoted reads a quoted run starting at sql[i] (the opening quote),
// where a doubled quote stands for a literal one. It returns the contents
// and the index after the closing quote.
func readQuoted(sql string, i int, quote byte) (string, int) {
	var b strings.Builder
	i++
	for i < len(sql) {
		if sql[i] == quote {
			if i+1 < len(sql) && sql[i+1] == quote {
				b.WriteByte(quote)
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

func isIdentStart(c byte) bool {
	return c == '_' || (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || c >= 0x80
}

func isIdentChar(c byte) bool {
	return isIdentStart(c) || c == '$' || (c >= '0' && c <= '9')
}
