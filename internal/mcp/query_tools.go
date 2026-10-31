package mcp

import (
	"context"
	"errors"
	"fmt"
	"strings"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/marmotdata/marmot/internal/api/v1/common"
	"github.com/marmotdata/marmot/internal/core/dataquery"
	"github.com/marmotdata/marmot/internal/telemetry/lookups"
)

// ListQueryableSourcesInput optionally narrows the list by name or plugin.
type ListQueryableSourcesInput struct {
	Search string `json:"search,omitempty"`
}

// sourcesPageSize caps how many sources one tool call lists, so a large
// deployment does not flood the agent's context.
const sourcesPageSize = 50

// QueryDataInput is one read-only SQL statement to run against a source.
type QueryDataInput struct {
	Source    string `json:"source"`
	Statement string `json:"statement"`
	MaxRows   int64  `json:"max_rows,omitempty"`
}

// canQuery asks the data query authorizer whether the caller may query at all.
func (tc *ToolContext) canQuery(ctx context.Context) bool {
	return tc.principal != nil && tc.queryService != nil && tc.queryService.Admits(ctx, tc.principal)
}

func (tc *ToolContext) listQueryableSources(
	ctx context.Context,
	req *mcpsdk.CallToolRequest,
	args ListQueryableSourcesInput,
) (*mcpsdk.CallToolResult, any, error) {
	if !tc.canQuery(ctx) {
		return tc.errorWithGuidance("Querying data is not available to you", "Running queries requires the assets:query permission.", nil), nil, nil
	}

	sources, total, err := tc.queryService.ListSources(ctx, dataquery.SourceFilter{Search: args.Search, Limit: sourcesPageSize, Principal: tc.principal})
	if err != nil {
		return tc.errorWithGuidance("Could not list queryable sources", err.Error(), nil), nil, nil
	}
	if len(sources) == 0 && args.Search != "" {
		return textResult(fmt.Sprintf("No queryable source matches %q. Try a shorter search, or call with no search to see them all.", args.Search)), nil, nil
	}
	if len(sources) == 0 {
		return textResult("No sources answer queries yet. An administrator can switch on \"Answer agent queries\" for a source in Marmot."), nil, nil
	}

	var b strings.Builder
	b.WriteString("# Queryable sources\n\n")
	for _, s := range sources {
		fmt.Fprintf(&b, "- **%s** (%s)\n", s.Name, s.PluginID)
	}
	if total > len(sources) {
		fmt.Fprintf(&b, "\nShowing %d of %d sources. Pass \"search\" with part of a name or plugin to narrow the list.\n", len(sources), total)
	}
	b.WriteString("\nRun a query with query_data, passing one of these names as \"source\". Use discover_data first to find the tables you need and how they are described.")
	return textResult(b.String()), nil, nil
}

func (tc *ToolContext) queryData(
	ctx context.Context,
	req *mcpsdk.CallToolRequest,
	args QueryDataInput,
) (*mcpsdk.CallToolResult, any, error) {
	if !tc.canQuery(ctx) {
		return tc.errorWithGuidance("Querying data is not available to you", "Running queries requires the assets:query permission.", nil), nil, nil
	}
	if strings.TrimSpace(args.Source) == "" || strings.TrimSpace(args.Statement) == "" {
		return tc.errorWithGuidance(
			"Both source and statement are required",
			"Call list_queryable_sources to find a source name, then pass one read-only SQL statement.",
			map[string]string{
				"Example": `{"source": "warehouse", "statement": "SELECT * FROM public.orders LIMIT 10"}`,
			},
		), nil, nil
	}

	input := dataquery.QueryInput{
		Source:    args.Source,
		Statement: args.Statement,
		MaxRows:   args.MaxRows,
	}
	// The MCP client names itself, which is the agent behind the caller's identity in the query log.
	if info := clientInfo(req); info != nil {
		input.Agent = info.Name
	}
	if credential, ok := common.CredentialFromContext(ctx); ok {
		input.Credential = credential.Type
	}
	result, err := tc.queryService.Query(ctx, tc.principal, input, dataquery.ClientMCP)
	if err != nil {
		switch {
		case errors.Is(err, dataquery.ErrDenied):
			return tc.errorWithGuidance(
				"Access denied",
				err.Error()+"\n\nDon't rewrite the query to get round this. Tell the user which access is missing so an administrator can give your identity access to the data this statement reads.",
				nil,
			), nil, nil
		case errors.Is(err, dataquery.ErrSourceNotFound):
			return tc.errorWithGuidance("Unknown source", "Call list_queryable_sources to see the sources you can query.", nil), nil, nil
		default:
			return tc.errorWithGuidance(
				"Query failed",
				err.Error(),
				map[string]string{
					"Read only": "Only read statements run; writes are refused",
					"Dialect":   "Write the statement in the source's own SQL dialect, e.g. catalog.schema.table for Trino",
				},
			), nil, nil
		}
	}

	tc.recordLookup(ctx, lookups.CategoryAssetDetail)
	return textResult(renderQueryResult(result)), nil, nil
}

// renderQueryResult formats rows as a markdown table followed by the
// catalog's description of every table the query read, so the agent gets
// meaning alongside the data.
func renderQueryResult(result *dataquery.QueryResult) string {
	var b strings.Builder

	fmt.Fprintf(&b, "# Query result: %d row(s)", result.RowCount)
	if result.Truncated {
		b.WriteString(", truncated at the row limit")
	}
	b.WriteString("\n\n")

	if len(result.Columns) > 0 {
		header := make([]string, len(result.Columns))
		for i, c := range result.Columns {
			header[i] = formatCell(c.Name)
		}
		b.WriteString("| " + strings.Join(header, " | ") + " |\n")
		b.WriteString("|" + strings.Repeat(" --- |", len(header)) + "\n")

		for _, row := range result.Rows {
			cells := make([]string, len(header))
			for i := range header {
				if i < len(row) {
					cells[i] = formatCell(row[i])
				}
			}
			b.WriteString("| " + strings.Join(cells, " | ") + " |\n")
		}
	}

	if len(result.Context) > 0 {
		b.WriteString("\n## What the catalog says about the tables read\n\n")
		for _, c := range result.Context {
			fmt.Fprintf(&b, "- **%s**", c.Name)
			if c.Type != "" {
				fmt.Fprintf(&b, " (%s)", c.Type)
			}
			if c.Description != "" {
				fmt.Fprintf(&b, ": %s", c.Description)
			}
			if len(c.Tags) > 0 {
				fmt.Fprintf(&b, " · tags: %s", strings.Join(c.Tags, ", "))
			}
			fmt.Fprintf(&b, " · `%s`\n", c.MRN)
		}
	}

	fmt.Fprintf(&b, "\n_Recorded in Marmot's query log as %s._", result.QueryID)
	return b.String()
}

func formatCell(v any) string {
	if v == nil {
		return ""
	}
	s := fmt.Sprintf("%v", v)
	// Keep each row on one line.
	s = strings.ReplaceAll(s, "\r", " ")
	s = strings.ReplaceAll(s, "\n", " ")
	return strings.ReplaceAll(s, "|", "\\|")
}

func textResult(text string) *mcpsdk.CallToolResult {
	return &mcpsdk.CallToolResult{
		Content: []mcpsdk.Content{&mcpsdk.TextContent{Text: text}},
	}
}

// clientInfo reads the calling MCP client's self-description, which tests and in-process callers may not have.
func clientInfo(req *mcpsdk.CallToolRequest) *mcpsdk.Implementation {
	if req == nil {
		return nil
	}
	return req.ClientInfo()
}
