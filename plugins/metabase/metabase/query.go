package metabase

import (
	"bytes"
	"encoding/json"
	"regexp"
	"strconv"
	"strings"
)

// A card's dataset_query has had two shapes. Up to 0.56 it was legacy
// MBQL:
//
//	{"type": "native", "native": {"query": "SELECT ..."}}
//	{"type": "query", "query": {"source-table": 10, "joins": [{"source-table": 12}]}}
//
// From 0.57 the API returns pMBQL, a list of stages:
//
//	{"stages": [{"lib/type": "mbql.stage/native", "native": "SELECT ..."}]}
//	{"stages": [{"lib/type": "mbql.stage/mbql", "source-table": 10,
//	             "joins": [{"stages": [{"source-table": 12}]}]}]}
//
// A saved question used as a source is "card__42" in legacy
// source-table and an integer source-card in pMBQL. Only the first
// stage carries a source; later stages refine the previous one.
type datasetQuery struct {
	Type   string       `json:"type"`
	Native *nativeQuery `json:"native"`
	Query  *queryStage  `json:"query"`
	Stages []queryStage `json:"stages"`
}

type nativeQuery struct {
	Query string `json:"query"`
}

// queryStage is a legacy MBQL query object or one pMBQL stage; the two
// share their field names.
type queryStage struct {
	LibType     string          `json:"lib/type"`
	Native      json.RawMessage `json:"native"`
	SourceTable json.RawMessage `json:"source-table"`
	SourceCard  int             `json:"source-card"`
	Joins       []queryJoin     `json:"joins"`
}

// queryJoin is a legacy join, which names its source directly, or a
// pMBQL join, which nests it in stages.
type queryJoin struct {
	SourceTable json.RawMessage `json:"source-table"`
	SourceCard  int             `json:"source-card"`
	Stages      []queryStage    `json:"stages"`
}

// querySource is one input of a card: a synced table or another card.
type querySource struct {
	TableID int
	CardID  int
}

func parseDatasetQuery(raw json.RawMessage) (datasetQuery, error) {
	var q datasetQuery
	if len(bytes.TrimSpace(raw)) == 0 {
		return q, nil
	}
	err := json.Unmarshal(raw, &q)
	return q, err
}

// firstStage returns the stage that names the card's source, in either
// shape, or nil for a native query or an empty query.
func (q datasetQuery) firstStage() *queryStage {
	if q.Query != nil {
		return q.Query
	}
	if len(q.Stages) > 0 && q.Stages[0].LibType != "mbql.stage/native" {
		return &q.Stages[0]
	}
	return nil
}

// sql returns the SQL of a native card, or "" for an MBQL card. A
// legacy native query nests it under native.query; a pMBQL native
// stage carries it as the native string itself.
func (q datasetQuery) sql() string {
	if q.Native != nil {
		return strings.TrimSpace(q.Native.Query)
	}
	for _, stage := range q.Stages {
		if len(stage.Native) == 0 {
			continue
		}
		var text string
		if err := json.Unmarshal(stage.Native, &text); err == nil {
			return strings.TrimSpace(text)
		}
		var nested nativeQuery
		if err := json.Unmarshal(stage.Native, &nested); err == nil {
			return strings.TrimSpace(nested.Query)
		}
	}
	return ""
}

// isNative reports whether the card runs SQL rather than MBQL.
func (q datasetQuery) isNative() bool {
	if q.Type != "" {
		return q.Type == "native"
	}
	return len(q.Stages) > 0 && q.Stages[0].LibType == "mbql.stage/native"
}

// sources returns the tables and cards an MBQL card reads from: its
// source and the source of every join. Native cards resolve their
// sources from the SQL instead.
func (q datasetQuery) sources() []querySource {
	stage := q.firstStage()
	if stage == nil {
		return nil
	}

	var sources []querySource
	add := func(rawTable json.RawMessage, cardID int) {
		if src, ok := sourceFrom(rawTable, cardID); ok {
			sources = append(sources, src)
		}
	}

	add(stage.SourceTable, stage.SourceCard)
	for _, join := range stage.Joins {
		if len(join.Stages) > 0 {
			add(join.Stages[0].SourceTable, join.Stages[0].SourceCard)
			continue
		}
		add(join.SourceTable, join.SourceCard)
	}
	return sources
}

// sourceFrom decodes a source-table value, an integer table id or a
// "card__<id>" string, alongside a pMBQL source-card id.
func sourceFrom(rawTable json.RawMessage, cardID int) (querySource, bool) {
	if cardID != 0 {
		return querySource{CardID: cardID}, true
	}
	trimmed := bytes.TrimSpace(rawTable)
	if len(trimmed) == 0 || bytes.Equal(trimmed, []byte("null")) {
		return querySource{}, false
	}

	var tableID int
	if err := json.Unmarshal(trimmed, &tableID); err == nil && tableID != 0 {
		return querySource{TableID: tableID}, true
	}

	var text string
	if err := json.Unmarshal(trimmed, &text); err == nil {
		if id, err := strconv.Atoi(strings.TrimPrefix(text, "card__")); err == nil && strings.HasPrefix(text, "card__") {
			return querySource{CardID: id}, true
		}
	}
	return querySource{}, false
}

var (
	// optionalClauseRe matches Metabase's [[ ... ]] optional SQL
	// clauses, which only apply when their template tag has a value.
	optionalClauseRe = regexp.MustCompile(`\[\[[\s\S]*?\]\]`)
	// cardTagRe matches {{#42}} and {{#42-slug}}, a saved question
	// referenced from native SQL.
	cardTagRe = regexp.MustCompile(`\{\{\s*#(\d+)[^}]*\}\}`)
	// templateTagRe matches every remaining {{ ... }} variable.
	templateTagRe = regexp.MustCompile(`\{\{[^}]*\}\}`)
	lineCommentRe = regexp.MustCompile(`--[^\n]*`)
	blockComment  = regexp.MustCompile(`/\*[\s\S]*?\*/`)

	// identifier is one name part: quoted with double quotes, backticks
	// or brackets, or bare.
	identifier = "(?:\"[^\"]+\"|`[^`]+`|\\[[^\\]]+\\]|[A-Za-z_][A-Za-z0-9_$]*)"
	// tableRefRe matches the name after FROM or JOIN, up to three parts.
	// The trailing group looks past the name so a call like
	// FROM generate_series(1, 10) can be told apart from a table.
	tableRefRe = regexp.MustCompile(`(?i)\b(?:FROM|JOIN)\s+(` + identifier + `(?:\s*\.\s*` + identifier + `){0,2})\s*(\(?)`)
	// cteRe matches a common table expression name: "name AS (".
	cteRe = regexp.MustCompile(`(?i)\b(` + identifier + `)\s*(?:\([^)]*\)\s*)?AS\s*\(`)
)

// tableRef is a table named in SQL, split into its dotted parts with
// quoting removed. Schema is empty for an unqualified name.
type tableRef struct {
	Schema string
	Name   string
}

// sqlTableRefs extracts the tables a native query reads from, and the
// ids of saved questions it references through {{#id}} tags. It is a
// conservative scan rather than a parser: it takes the name after
// every FROM and JOIN, drops names defined by a WITH clause, and
// ignores subqueries and function calls. Names it cannot make sense of
// simply produce no reference.
func sqlTableRefs(sql string) ([]tableRef, []int) {
	var cardIDs []int
	for _, m := range cardTagRe.FindAllStringSubmatch(sql, -1) {
		if id, err := strconv.Atoi(m[1]); err == nil {
			cardIDs = append(cardIDs, id)
		}
	}

	// A template tag standing in for a table, FROM {{#42}} m, must not
	// leave its alias behind as the name after FROM, so tags become a
	// parenthesised placeholder that the scan treats as a subquery.
	cleaned := optionalClauseRe.ReplaceAllString(sql, " ")
	cleaned = templateTagRe.ReplaceAllString(cleaned, "(?)")
	cleaned = blockComment.ReplaceAllString(cleaned, " ")
	cleaned = lineCommentRe.ReplaceAllString(cleaned, " ")

	ctes := make(map[string]struct{})
	for _, m := range cteRe.FindAllStringSubmatch(cleaned, -1) {
		ctes[strings.ToLower(unquote(m[1]))] = struct{}{}
	}

	var refs []tableRef
	seen := make(map[tableRef]struct{})
	for _, m := range tableRefRe.FindAllStringSubmatch(cleaned, -1) {
		if m[2] == "(" {
			continue
		}
		parts := splitIdentifier(m[1])
		if len(parts) == 0 {
			continue
		}
		ref := tableRef{Name: parts[len(parts)-1]}
		if len(parts) > 1 {
			ref.Schema = parts[len(parts)-2]
		}
		if ref.Schema == "" {
			if _, isCTE := ctes[strings.ToLower(ref.Name)]; isCTE {
				continue
			}
		}
		if _, dup := seen[ref]; dup {
			continue
		}
		seen[ref] = struct{}{}
		refs = append(refs, ref)
	}
	return refs, cardIDs
}

// splitIdentifier splits a.b.c into its parts, honouring quotes so a
// dot inside a quoted name does not split it.
func splitIdentifier(name string) []string {
	var parts []string
	var current strings.Builder
	var quote rune
	for _, r := range name {
		switch {
		case quote != 0:
			if r == quote || (quote == '[' && r == ']') {
				quote = 0
			} else {
				current.WriteRune(r)
			}
		case r == '"' || r == '`' || r == '[':
			quote = r
		case r == '.':
			parts = append(parts, current.String())
			current.Reset()
		case r == ' ' || r == '\t' || r == '\n' || r == '\r':
			// Whitespace around the dots is allowed by the regex.
		default:
			current.WriteRune(r)
		}
	}
	parts = append(parts, current.String())
	return parts
}

func unquote(name string) string {
	parts := splitIdentifier(name)
	return parts[len(parts)-1]
}
