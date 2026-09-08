package questdb

import (
	"strings"
	"unicode"
)

// viewReferences returns the table names a view's SQL reads FROM or JOINs,
// in order of first appearance and without duplicates. Subqueries and
// function calls in those positions are skipped. QuestDB has no schema
// qualifier, so a name is taken whole, quotes stripped.
func viewReferences(sql string) []string {
	tokens := tokenize(stripComments(sql))

	var refs []string
	seen := make(map[string]struct{})

	for i := 0; i+1 < len(tokens); i++ {
		if !tokens[i].isKeyword("from") && !tokens[i].isKeyword("join") {
			continue
		}

		// FROM a [AS] x, b [AS] y: walk the comma-separated list.
		j := i + 1
		for j < len(tokens) {
			name := tokens[j]
			if !name.isName() || (j+1 < len(tokens) && tokens[j+1].text == "(") {
				break
			}

			key := strings.ToLower(name.text)
			if _, dup := seen[key]; !dup {
				seen[key] = struct{}{}
				refs = append(refs, name.text)
			}
			j++

			if j < len(tokens) && tokens[j].isKeyword("as") {
				j++
			}
			if j < len(tokens) && tokens[j].isName() {
				j++ // alias
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

type token struct {
	text   string
	quoted bool
}

func (t token) isKeyword(word string) bool {
	return !t.quoted && strings.EqualFold(t.text, word)
}

// isName reports whether the token can be a table name or alias: anything
// quoted, or a bare word that is not a reserved keyword or punctuation.
func (t token) isName() bool {
	if t.quoted {
		return true
	}
	if t.text == "" || !(isWordRune(rune(t.text[0]))) {
		return false
	}
	_, reserved := sqlKeywords[strings.ToLower(t.text)]
	return !reserved
}

// sqlKeywords are the words that can follow a table name or alias in a
// QuestDB query, so they are never mistaken for a name.
var sqlKeywords = map[string]struct{}{
	"as": {}, "asof": {}, "by": {}, "cross": {}, "distinct": {}, "except": {},
	"from": {}, "full": {}, "group": {}, "having": {}, "inner": {},
	"intersect": {}, "join": {}, "latest": {}, "left": {}, "limit": {},
	"lt": {}, "natural": {}, "on": {}, "order": {}, "outer": {},
	"partition": {}, "right": {}, "sample": {}, "select": {}, "splice": {},
	"timestamp": {}, "union": {}, "using": {}, "where": {}, "window": {},
	"with": {},
}

func isWordRune(r rune) bool {
	return r == '_' || r == '.' || r == '$' || unicode.IsLetter(r) || unicode.IsDigit(r)
}

// tokenize splits SQL into bare words, quoted names and single-character
// punctuation. Quoting uses double quotes or, as QuestDB also allows for
// table names, single quotes; a doubled quote inside is an escape.
func tokenize(sql string) []token {
	var tokens []token
	runes := []rune(sql)

	for i := 0; i < len(runes); {
		r := runes[i]
		switch {
		case unicode.IsSpace(r):
			i++
		case r == '"' || r == '\'':
			var text strings.Builder
			i++
			for i < len(runes) {
				if runes[i] == r {
					if i+1 < len(runes) && runes[i+1] == r {
						text.WriteRune(r)
						i += 2
						continue
					}
					i++
					break
				}
				text.WriteRune(runes[i])
				i++
			}
			tokens = append(tokens, token{text: text.String(), quoted: true})
		case isWordRune(r):
			start := i
			for i < len(runes) && isWordRune(runes[i]) {
				i++
			}
			tokens = append(tokens, token{text: string(runes[start:i])})
		default:
			tokens = append(tokens, token{text: string(r)})
			i++
		}
	}

	return tokens
}

// stripComments removes -- line comments and /* */ block comments.
func stripComments(sql string) string {
	var out strings.Builder
	for i := 0; i < len(sql); {
		switch {
		case strings.HasPrefix(sql[i:], "--"):
			end := strings.Index(sql[i:], "\n")
			if end < 0 {
				return out.String()
			}
			i += end
		case strings.HasPrefix(sql[i:], "/*"):
			end := strings.Index(sql[i:], "*/")
			if end < 0 {
				return out.String()
			}
			out.WriteByte(' ')
			i += end + 2
		default:
			out.WriteByte(sql[i])
			i++
		}
	}
	return out.String()
}
