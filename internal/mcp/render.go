package mcp

import (
	"encoding/json"
	"fmt"
	"net/url"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/marmotdata/marmot/internal/core/asset"
	"github.com/marmotdata/marmot/internal/core/knowledge"
	"github.com/marmotdata/plugin-sdk/mrn"
	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

// Every tool answer is plain Markdown built from these helpers, so identifiers, links, next steps and errors read the same way whichever tool produced them.

// Response budgets keep one answer well under the 10,000-token size Claude Code warns about; longer content says what was left out and how to ask for it.
const (
	cardBudget        = 24000
	docsBudget        = 9000
	maxSchemaColumns  = 60
	maxMetadataKeys   = 30
	maxMetadataValue  = 200
	maxQueryText      = 2000
	maxListSummary    = 160
	maxLineageInCard  = 12
	maxLineageInTrace = 60
)

// step is one suggested next call: a short reason, the tool and its real arguments, never a placeholder.
type step struct {
	why  string
	tool string
	args map[string]any
}

// call renders a tool call the way an agent should send it.
func (s step) call() string {
	if len(s.args) == 0 {
		return s.tool + " {}"
	}
	return s.tool + " " + compactJSON(s.args)
}

// compactJSON marshals with sorted keys (encoding/json sorts map keys) and no HTML escaping, so names with & or < read as written.
func compactJSON(v any) string {
	var b strings.Builder
	enc := json.NewEncoder(&b)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		return "{}"
	}
	return strings.TrimSpace(b.String())
}

// writeNext appends the next steps in the order given, dropping repeats.
func writeNext(b *strings.Builder, steps ...step) {
	seen := map[string]bool{}
	var lines []string
	for _, s := range steps {
		if s.tool == "" {
			continue
		}
		c := s.call()
		if seen[c] {
			continue
		}
		seen[c] = true
		lines = append(lines, fmt.Sprintf("- %s: %s", s.why, c))
	}
	if len(lines) == 0 {
		return
	}
	b.WriteString("\n\nNext:\n")
	b.WriteString(strings.Join(lines, "\n"))
}

// Error codes tell an agent whether to retry, fix its arguments or stop.
const (
	codeNotFound        = "not_found"
	codeInvalidArgument = "invalid_argument"
	codeAmbiguous       = "ambiguous"
	codePermission      = "permission_denied"
	codeUnavailable     = "unavailable"
)

// toolError is a failed call: a code, one line saying what went wrong and what to do, and real calls to try next.
func toolError(code, message string, next ...step) *mcpsdk.CallToolResult {
	var b strings.Builder
	fmt.Fprintf(&b, "Error (%s): %s", code, message)
	writeNext(&b, next...)
	return &mcpsdk.CallToolResult{Content: []mcpsdk.Content{&mcpsdk.TextContent{Text: b.String()}}, IsError: true}
}

// unavailable is the answer when a backend call failed; the cause is logged by the caller, never shown, and the agent is told a retry may work.
func unavailable(what string) *mcpsdk.CallToolResult {
	return toolError(codeUnavailable, fmt.Sprintf("Marmot couldn't %s just now. Retry the same call once; if it fails again, tell the user Marmot is having trouble.", what))
}

// truncate cuts s to at most n runes on a rune boundary, adding an ellipsis when it cut anything.
func truncate(s string, n int) string {
	if n <= 0 || utf8.RuneCountInString(s) <= n {
		return s
	}
	r := []rune(s)
	return strings.TrimRight(string(r[:n-1]), " ") + "…"
}

// oneLine flattens whitespace so a description fits on a list line.
func oneLine(s string) string {
	return strings.Join(strings.Fields(s), " ")
}

// sortedKeys returns a map's keys in order, so every answer comes out the same way twice.
func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// byCount orders a facet map largest first, then by name.
func byCount(m map[string]int) []string {
	keys := sortedKeys(m)
	sort.SliceStable(keys, func(i, j int) bool { return m[keys[i]] > m[keys[j]] })
	return keys
}

// facetLine renders a facet map as "Table 12, Topic 3", largest first, cut to limit entries.
func facetLine(m map[string]int, limit int) string {
	keys := byCount(m)
	more := 0
	if limit > 0 && len(keys) > limit {
		more = len(keys) - limit
		keys = keys[:limit]
	}
	parts := make([]string, len(keys))
	for i, k := range keys {
		parts[i] = fmt.Sprintf("%s %d", k, m[k])
	}
	s := strings.Join(parts, ", ")
	if more > 0 {
		s += fmt.Sprintf(" and %d more", more)
	}
	return s
}

// assetURL is the page for an asset in the Marmot UI: a knowledge entry opens under Knowledge, anything else at /discover/<type>/<provider[@zone]>/<name> exactly as the UI builds it from the MRN.
func assetURL(root string, a *asset.Asset) string {
	if root == "" || a == nil {
		return ""
	}
	root = strings.TrimRight(root, "/")
	if _, ok := knowledge.KindOfType(a.Type); ok {
		return root + "/knowledge/" + url.PathEscape(a.ID)
	}
	if a.MRN != nil {
		if m, err := mrn.ParseMRN(*a.MRN); err == nil && m.Kind == mrn.KindAsset {
			return fmt.Sprintf("%s/discover/%s/%s/%s", root, routeSegment(m.Type), routeSegment(m.ProviderSegment()), routeSegment(m.Name))
		}
	}
	provider := ""
	if len(a.Providers) > 0 {
		provider = a.Providers[0]
	}
	clean := func(s string) string { return strings.NewReplacer("/", "-", " ", "-").Replace(strings.ToLower(s)) }
	return fmt.Sprintf("%s/discover/%s/%s/%s", root, routeSegment(clean(a.Type)), routeSegment(clean(provider)), routeSegment(clean(deref(a.Name))))
}

// routeSegment encodes one path segment the way the UI does, keeping @ readable.
func routeSegment(s string) string {
	return strings.ReplaceAll(url.PathEscape(s), "%40", "@")
}

func termURL(root, id string) string {
	if root == "" {
		return ""
	}
	return strings.TrimRight(root, "/") + "/glossary/" + url.PathEscape(id)
}

func productURL(root, id string) string {
	if root == "" {
		return ""
	}
	return strings.TrimRight(root, "/") + "/products/" + url.PathEscape(id)
}

func teamURL(root, id string) string {
	if root == "" {
		return ""
	}
	return strings.TrimRight(root, "/") + "/teams/" + url.PathEscape(id)
}

// assetLine is one asset on a list: name, type and provider, then its MRN, which is the id every tool takes, and a short description.
func assetLine(a *asset.Asset, root string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "- %s (%s", deref(a.Name), a.Type)
	if len(a.Providers) > 0 {
		fmt.Fprintf(&b, ", %s", a.Providers[0])
	}
	b.WriteString(")")
	if a.IsStub {
		b.WriteString(" [placeholder from lineage, not catalogued]")
	}
	if deprecated(a) {
		b.WriteString(" [deprecated]")
	}
	if a.MRN != nil {
		fmt.Fprintf(&b, "\n  mrn: %s", *a.MRN)
	}
	if u := assetURL(root, a); u != "" {
		fmt.Fprintf(&b, "\n  url: %s", u)
	}
	if d := summaryText(a); d != "" {
		fmt.Fprintf(&b, "\n  %s", truncate(oneLine(d), maxListSummary))
	}
	return b.String()
}

// summaryText prefers what people wrote about an asset over what its source says, matching the UI.
func summaryText(a *asset.Asset) string {
	if a.UserDescription != nil && strings.TrimSpace(*a.UserDescription) != "" {
		return *a.UserDescription
	}
	return deref(a.Description)
}

func deprecated(a *asset.Asset) bool {
	for _, t := range a.Tags {
		if strings.EqualFold(t, "deprecated") {
			return true
		}
	}
	return false
}

// idFor is the identifier to hand back to a tool: the MRN when there is one, since it is stable and readable, else the ID.
func idFor(a *asset.Asset) string {
	if a.MRN != nil && *a.MRN != "" {
		return *a.MRN
	}
	return a.ID
}

// updatedText resolves an update time for the reader, such as "3 days ago (2026-11-20)".
func updatedText(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return fmt.Sprintf("%s (%s)", ago(t), t.Format("2006-01-02"))
}
