package starrocks

import (
	"regexp"
	"strings"
)

// queryReferences returns the object names a query reads: whatever
// follows FROM or JOIN, including comma-separated lists with aliases.
// Names keep their qualifiers ("shop.orders") and lose their backticks.
// Subqueries and CTE names come out too or not at all; the caller only
// keeps names that resolve to an object discovered in the same run, so
// a stray match costs nothing.
func queryReferences(query string) []string {
	tokens := tokenize(query)
	var refs []string
	seen := make(map[string]struct{})

	for i := 0; i < len(tokens); i++ {
		if !isKeyword(tokens[i], "FROM") && !isKeyword(tokens[i], "JOIN") {
			continue
		}

		pos := i + 1
		for {
			name, next := readQualifiedName(tokens, pos)
			if name == "" {
				break
			}
			if _, ok := seen[name]; !ok {
				seen[name] = struct{}{}
				refs = append(refs, name)
			}

			next = skipAlias(tokens, next)
			if next < len(tokens) && tokens[next] == "," {
				pos = next + 1
				continue
			}
			break
		}
	}

	return refs
}

// tokenRe splits SQL into comments, string literals, quoted identifiers,
// words and single punctuation characters, in that priority order.
var tokenRe = regexp.MustCompile("(?s)--[^\\n]*|#[^\\n]*|/\\*.*?\\*/|'(?:[^'\\\\]|\\\\.)*'|\"(?:[^\"\\\\]|\\\\.)*\"|`[^`]*`|[A-Za-z_$][A-Za-z0-9_$]*|[0-9][A-Za-z0-9_.]*|[^\\s]")

// tokenize drops comments and string literals, since neither can hold a
// table reference, and strips backticks from quoted identifiers.
func tokenize(query string) []string {
	var tokens []string
	for _, tok := range tokenRe.FindAllString(query, -1) {
		switch {
		case strings.HasPrefix(tok, "--"), strings.HasPrefix(tok, "#"), strings.HasPrefix(tok, "/*"):
			continue
		case strings.HasPrefix(tok, "'"), strings.HasPrefix(tok, "\""):
			tokens = append(tokens, "'")
		case strings.HasPrefix(tok, "`"):
			tokens = append(tokens, strings.Trim(tok, "`"))
		default:
			tokens = append(tokens, tok)
		}
	}
	return tokens
}

// readQualifiedName reads ident(.ident)* starting at pos and returns the
// dotted name and the position after it. It returns "" when pos does not
// start an identifier, for example at a subquery's opening parenthesis.
func readQualifiedName(tokens []string, pos int) (string, int) {
	if pos >= len(tokens) || !isIdentifier(tokens[pos]) {
		return "", pos
	}
	parts := []string{tokens[pos]}
	pos++
	for pos+1 < len(tokens) && tokens[pos] == "." && isIdentifier(tokens[pos+1]) {
		parts = append(parts, tokens[pos+1])
		pos += 2
	}
	return strings.Join(parts, "."), pos
}

// skipAlias steps over an optional "AS alias" or bare alias after a name.
func skipAlias(tokens []string, pos int) int {
	if pos < len(tokens) && isKeyword(tokens[pos], "AS") {
		pos++
		if pos < len(tokens) && isIdentifier(tokens[pos]) {
			pos++
		}
		return pos
	}
	if pos < len(tokens) && isIdentifier(tokens[pos]) && !isReservedWord(tokens[pos]) {
		pos++
	}
	return pos
}

func isKeyword(token, keyword string) bool {
	return strings.EqualFold(token, keyword)
}

func isIdentifier(token string) bool {
	if token == "" || token == "'" {
		return false
	}
	c := token[0]
	return c == '_' || c == '$' || (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z')
}

// reservedWords are the words that can follow a table name without being
// its alias.
var reservedWords = map[string]struct{}{
	"WHERE": {}, "GROUP": {}, "ORDER": {}, "LIMIT": {}, "HAVING": {}, "UNION": {},
	"JOIN": {}, "LEFT": {}, "RIGHT": {}, "INNER": {}, "OUTER": {}, "FULL": {},
	"CROSS": {}, "NATURAL": {}, "ON": {}, "USING": {}, "SET": {}, "QUALIFY": {},
	"WINDOW": {}, "INTERSECT": {}, "EXCEPT": {}, "MINUS": {}, "FOR": {}, "INTO": {},
	"WITH": {}, "SELECT": {}, "FROM": {}, "AS": {}, "LATERAL": {}, "PARTITION": {},
	"TABLESAMPLE": {}, "SEMI": {}, "ANTI": {}, "STRAIGHT_JOIN": {},
}

func isReservedWord(token string) bool {
	_, ok := reservedWords[strings.ToUpper(token)]
	return ok
}

// resolveReference turns a name from a query into (database, object).
// A bare name lives in the query's own database; a two-part name carries
// its database; a three-part name must be in the configured catalog or
// it points outside this run.
func resolveReference(ref, catalog, currentDB string) (db, object string, ok bool) {
	parts := splitQualifiedName(ref)
	switch len(parts) {
	case 1:
		return currentDB, parts[0], true
	case 2:
		return parts[0], parts[1], true
	case 3:
		if !strings.EqualFold(parts[0], catalog) {
			return "", "", false
		}
		return parts[1], parts[2], true
	}
	return "", "", false
}
