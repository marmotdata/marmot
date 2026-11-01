package hive

import (
	"strings"
)

// viewReferences returns the db.table names a view's query reads, in order
// of appearance and without duplicates. Unqualified names take the view's
// own database, which is how Hive resolves them. The scan is lexical: it
// takes the name after every FROM and JOIN, follows a comma-separated FROM
// list, and skips subqueries. Names that turn out not to be tables (a CTE,
// for example) are dropped later, when references are resolved against the
// objects discovery actually found.
func viewReferences(query, defaultDB string) []string {
	tokens := tokenize(query)

	var refs []string
	seen := make(map[string]bool)
	add := func(ident string) {
		ref := qualify(ident, defaultDB)
		if ref == "" || seen[ref] {
			return
		}
		seen[ref] = true
		refs = append(refs, ref)
	}

	for i := 0; i < len(tokens); i++ {
		tok := tokens[i]
		if tok.quoted {
			continue
		}
		switch strings.ToUpper(tok.text) {
		case "JOIN":
			if i+1 < len(tokens) && tokens[i+1].isIdent() {
				add(tokens[i+1].text)
			}
		case "FROM":
			// FROM a x, b AS y WHERE ... : one reference per list entry.
			for i+1 < len(tokens) && tokens[i+1].isIdent() {
				add(tokens[i+1].text)
				i++
				// Skip the alias, with or without AS, up to the next comma.
				for i+1 < len(tokens) && tokens[i+1].isIdent() {
					if tokens[i+1].isKeyword() && !strings.EqualFold(tokens[i+1].text, "AS") {
						break
					}
					i++
				}
				if i+1 < len(tokens) && tokens[i+1].text == "," {
					i++
					continue
				}
				break
			}
		}
	}
	return refs
}

// qualify lowercases a table reference and prefixes the default database
// when it has none. A three-part name keeps its last two parts.
func qualify(ident, defaultDB string) string {
	parts := strings.Split(strings.ToLower(ident), ".")
	switch len(parts) {
	case 0:
		return ""
	case 1:
		if parts[0] == "" {
			return ""
		}
		return strings.ToLower(defaultDB) + "." + parts[0]
	default:
		return parts[len(parts)-2] + "." + parts[len(parts)-1]
	}
}

type token struct {
	text string
	// quoted marks a backtick-quoted identifier, which is never a keyword.
	quoted bool
	// ident marks an identifier or a keyword, as opposed to punctuation,
	// a number or a string literal.
	ident bool
}

func (t token) isIdent() bool { return t.ident }

// fromListTerminators are the words that end a FROM list, so an alias is
// never mistaken for one of them.
var fromListTerminators = map[string]bool{
	"WHERE": true, "GROUP": true, "ORDER": true, "HAVING": true, "LIMIT": true,
	"JOIN": true, "INNER": true, "LEFT": true, "RIGHT": true, "FULL": true,
	"CROSS": true, "NATURAL": true, "SEMI": true, "ANTI": true, "OUTER": true,
	"UNION": true, "INTERSECT": true, "EXCEPT": true, "MINUS": true,
	"ON": true, "USING": true, "LATERAL": true, "WINDOW": true,
	"CLUSTER": true, "DISTRIBUTE": true, "SORT": true, "TABLESAMPLE": true,
	"SELECT": true, "AS": true, "WITH": true,
}

func (t token) isKeyword() bool {
	return !t.quoted && fromListTerminators[strings.ToUpper(t.text)]
}

// tokenize splits HiveQL into identifiers (dotted and backtick-quoted parts
// kept together), string literals and single-character punctuation.
// Comments are dropped.
func tokenize(query string) []token {
	var tokens []token
	i := 0
	for i < len(query) {
		c := query[i]
		switch {
		case c == ' ' || c == '\t' || c == '\n' || c == '\r':
			i++
		case c == '-' && i+1 < len(query) && query[i+1] == '-':
			for i < len(query) && query[i] != '\n' {
				i++
			}
		case c == '/' && i+1 < len(query) && query[i+1] == '*':
			end := strings.Index(query[i+2:], "*/")
			if end < 0 {
				i = len(query)
			} else {
				i += end + 4
			}
		case c == '\'' || c == '"':
			end := i + 1
			for end < len(query) && query[end] != c {
				if query[end] == '\\' {
					end++
				}
				end++
			}
			tokens = append(tokens, token{text: query[i:min(end+1, len(query))]})
			i = end + 1
		case c == '`' || isIdentStart(c):
			text, quoted, next := readIdentifier(query, i)
			tokens = append(tokens, token{text: text, quoted: quoted, ident: true})
			i = next
		case c >= '0' && c <= '9':
			end := i
			for end < len(query) && (isIdentChar(query[end]) || query[end] == '.') {
				end++
			}
			tokens = append(tokens, token{text: query[i:end]})
			i = end
		default:
			tokens = append(tokens, token{text: string(c)})
			i++
		}
	}
	return tokens
}

// readIdentifier reads a possibly dotted identifier such as `sales`.`orders`
// or sales.orders starting at i, returning the text with backticks removed.
func readIdentifier(query string, i int) (text string, quoted bool, next int) {
	var parts []string
	for i < len(query) {
		if query[i] == '`' {
			quoted = true
			end := i + 1
			var part strings.Builder
			for end < len(query) {
				if query[end] == '`' {
					if end+1 < len(query) && query[end+1] == '`' {
						part.WriteByte('`')
						end += 2
						continue
					}
					break
				}
				part.WriteByte(query[end])
				end++
			}
			parts = append(parts, part.String())
			i = end + 1
		} else if isIdentStart(query[i]) || (len(parts) > 0 && isIdentChar(query[i])) {
			end := i
			for end < len(query) && isIdentChar(query[end]) {
				end++
			}
			parts = append(parts, query[i:end])
			i = end
		} else {
			break
		}
		if i < len(query) && query[i] == '.' {
			i++
			continue
		}
		break
	}
	return strings.Join(parts, "."), quoted, i
}

func isIdentStart(c byte) bool {
	return c == '_' || (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z')
}

func isIdentChar(c byte) bool {
	return isIdentStart(c) || (c >= '0' && c <= '9') || c == '$'
}
