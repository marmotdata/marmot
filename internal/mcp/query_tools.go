package mcp

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/marmotdata/marmot/internal/core/gateway"
	"github.com/marmotdata/marmot/internal/telemetry/lookups"
	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

// ListQueryTargetsInput takes no parameters.
type ListQueryTargetsInput struct{}

// QueryDataInput is a single SQL statement to run against a target.
type QueryDataInput struct {
	Target    string `json:"target"`
	Statement string `json:"statement"`
	MaxRows   int64  `json:"max_rows,omitempty"`
}

func (tc *ToolContext) listQueryTargets(
	ctx context.Context,
	req *mcpsdk.CallToolRequest,
	args ListQueryTargetsInput,
) (*mcpsdk.CallToolResult, any, error) {
	targets, err := tc.gatewayService.ListTargets(ctx)
	if err != nil {
		return tc.errorWithGuidance(
			"Could not list query targets",
			err.Error(),
			nil,
		), nil, nil
	}

	if len(targets) == 0 {
		return textResult("No query targets are configured. An administrator can add one under " +
			"Plugins → Query Targets before you can run queries."), nil, nil
	}

	var b strings.Builder
	b.WriteString("# Query targets\n\n")
	for _, t := range targets {
		status := ""
		if !t.Enabled {
			status = " (disabled)"
		}
		b.WriteString(fmt.Sprintf("- **%s**%s — %s engine, modes: %s\n", t.Name, status, t.PluginID, strings.Join(t.Modes, ", ")))
	}
	b.WriteString("\nRun a query with query_data, passing one of these names as \"target\".")

	return textResult(b.String()), nil, nil
}

func (tc *ToolContext) queryData(
	ctx context.Context,
	req *mcpsdk.CallToolRequest,
	args QueryDataInput,
) (*mcpsdk.CallToolResult, any, error) {
	if tc.principal == nil {
		return textResult("Query access is unavailable for this session."), nil, nil
	}
	if !tc.principal.IsAdmin() && !tc.principal.HasPermission("gateway", "query") {
		return textResult("You do not have permission to run queries through the gateway " +
			"(requires the gateway:query permission)."), nil, nil
	}
	if strings.TrimSpace(args.Target) == "" || strings.TrimSpace(args.Statement) == "" {
		return tc.errorWithGuidance(
			"Both target and statement are required",
			"Call list_query_targets to find a target name, then pass a SQL SELECT as statement.",
			map[string]string{
				"Example": `{"target": "trino-local", "statement": "SELECT * FROM postgresql.public.orders LIMIT 10"}`,
			},
		), nil, nil
	}

	result, err := tc.gatewayService.QueryForPrincipal(ctx, tc.principal, args.Target, args.Statement, args.MaxRows)
	if err != nil {
		if errors.Is(err, gateway.ErrDenied) {
			return textResult("Access denied: " + err.Error() +
				"\n\nAccess is granted per table. Ask an owner to grant your identity access to the " +
				"tables this query reads, or query only tables you are already granted."), nil, nil
		}
		return tc.errorWithGuidance(
			"Query failed",
			err.Error(),
			map[string]string{
				"List targets":  `{} to list_query_targets`,
				"Check dialect": "For Trino, qualify tables as catalog.schema.table",
			},
		), nil, nil
	}

	tc.recordLookup(ctx, lookups.CategoryAssetDetail)
	return textResult(renderQueryResult(result)), nil, nil
}

// renderQueryResult formats rows as a markdown table and appends the fused
// catalogue context so the agent receives meaning alongside the data.
func renderQueryResult(result *gateway.QueryResult) string {
	var b strings.Builder

	fmt.Fprintf(&b, "# Query result — %d row(s)", result.RowCount)
	if result.Truncated {
		b.WriteString(" (truncated to the row limit)")
	}
	b.WriteString("\n\n")

	if len(result.Columns) > 0 {
		header := make([]string, len(result.Columns))
		for i, c := range result.Columns {
			header[i] = c.Name
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
		b.WriteString("\n## Context for the tables this query read\n\n")
		for _, c := range result.Context {
			fmt.Fprintf(&b, "- **%s**", c.Name)
			if c.Type != "" {
				fmt.Fprintf(&b, " (%s)", c.Type)
			}
			if c.Description != "" {
				fmt.Fprintf(&b, " — %s", c.Description)
			}
			if len(c.Tags) > 0 {
				fmt.Fprintf(&b, " · tags: %s", strings.Join(c.Tags, ", "))
			}
			b.WriteString("\n")
		}
	}

	if len(result.ReferencedMRNs) > 0 {
		b.WriteString("\n_Audited. Referenced: " + strings.Join(result.ReferencedMRNs, ", ") + "_")
	}

	return b.String()
}

func formatCell(v any) string {
	if v == nil {
		return ""
	}
	s := fmt.Sprintf("%v", v)
	// Keep table rows on one line.
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.ReplaceAll(s, "|", "\\|")
	return s
}

// textResult wraps plain markdown text as a tool result.
func textResult(text string) *mcpsdk.CallToolResult {
	return &mcpsdk.CallToolResult{
		Content: []mcpsdk.Content{
			&mcpsdk.TextContent{Text: text},
		},
	}
}
