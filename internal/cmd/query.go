package cmd

import (
	"fmt"
	"io"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/marmotdata/marmot/internal/cmd/output"
	marmot "github.com/marmotdata/marmot/sdk/go"
	"github.com/spf13/cobra"
)

var queryCmd = &cobra.Command{
	Use:   "query",
	Short: "Run read-only SQL against catalogued data sources",
	Long: `Runs read-only SQL against catalogued sources as you, checks every table the
statement reads with the instance's access rules, records the query, and returns the rows together
with what the catalog knows about each table.

A typical session finds a source, looks at its tables, then queries one:

  marmot query sources --search shop
  marmot query tables shop-pg
  marmot query run shop-pg --table orders
  marmot query run shop-pg "SELECT status, count(*) FROM orders GROUP BY 1"`,
}

var querySourcesCmd = &cobra.Command{
	Use:   "sources",
	Short: "List the sources you can query",
	Example: `  marmot query sources
  marmot query sources --search warehouse -o json`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		search, _ := cmd.Flags().GetString("search")
		limit, _ := cmd.Flags().GetInt64("limit")
		offset, _ := cmd.Flags().GetInt64("offset")
		c, err := newClient()
		if err != nil {
			return err
		}
		list, err := c.Query.Sources(cmd.Context(), search, limit, offset)
		if err != nil {
			return err
		}
		return printOr(list, func(t *output.Table) {
			for _, s := range list.Sources {
				t.AddRow(s.Name, s.PluginID, strconv.FormatInt(s.TableCount, 10), strconv.FormatInt(s.QueriesLastWeek, 10), s.LastQueriedAt)
			}
			t.SetFooter("Showing %d of %d sources", len(list.Sources), list.Total)
		}, "NAME", "PLUGIN", "TABLES", "QUERIES 7D", "LAST QUERIED")
	},
}

var queryTablesCmd = &cobra.Command{
	Use:   "tables <source>",
	Short: "List the catalogued tables a source can reach, busiest first",
	Example: `  marmot query tables shop-pg
  marmot query tables shop-pg --search ord`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		search, _ := cmd.Flags().GetString("search")
		limit, _ := cmd.Flags().GetInt64("limit")
		c, err := newClient()
		if err != nil {
			return err
		}
		tables, err := c.Query.Tables(cmd.Context(), args[0], search, limit)
		if err != nil {
			return err
		}
		return printOr(tables, func(t *output.Table) {
			for _, tb := range tables {
				t.AddRow(tb.Name, tb.Type, tb.Schema, strconv.FormatInt(tb.Queries, 10), truncate(tb.Description, 60))
			}
			t.SetFooter("Query one with: marmot query run %s --table <name>", args[0])
		}, "NAME", "TYPE", "SCHEMA", "QUERIES 30D", "DESCRIPTION")
	},
}

var queryRunCmd = &cobra.Command{
	Use:   "run <source> [statement]",
	Short: "Run a read-only SQL statement against a source",
	Long: `Runs one read-only statement as you. The statement can be an argument, a file (--file),
standard input (pass - as the statement), or the preview of a table (--table), which writes
the right SELECT for the source's dialect.

You need the assets:query permission, and every table the statement reads must be one you
may query, otherwise the whole query is refused with the reason. Writes are always refused. The result
includes what the catalog knows about each table read, and the id the query was recorded
under in the query log.`,
	Example: `  marmot query run shop-pg --table orders
  marmot query run shop-pg "SELECT * FROM orders LIMIT 10"
  marmot query run trino --file report.sql --max-rows 1000 -o json
  echo "SELECT 1" | marmot query run shop-pg -`,
	Args: cobra.RangeArgs(1, 2),
	RunE: func(cmd *cobra.Command, args []string) error {
		file, _ := cmd.Flags().GetString("file")
		table, _ := cmd.Flags().GetString("table")
		maxRows, _ := cmd.Flags().GetInt64("max-rows")
		source := args[0]

		c, err := newClient()
		if err != nil {
			return err
		}

		statement, err := readStatement(args, file)
		if err != nil {
			return err
		}
		if statement == "" && table != "" {
			statement, err = previewStatement(cmd, c, source, table)
			if err != nil {
				return err
			}
		}
		if statement == "" {
			return usageError("no statement given; pass one as an argument, with --file, on stdin (-), or preview a table with --table (list them with: marmot query tables %s)", source)
		}

		res, err := c.Query.Run(cmd.Context(), marmot.QueryRunInput{Source: source, Statement: statement, MaxRows: maxRows})
		if err != nil {
			return err
		}

		p := getPrinter()
		if p.IsRaw() {
			data, err := marshalPayload(res)
			if err != nil {
				return err
			}
			return p.PrintRaw(data)
		}

		headers := make([]string, len(res.Columns))
		for i, col := range res.Columns {
			headers[i] = strings.ToUpper(col.Name)
		}
		t := output.NewTable(headers...)
		for _, row := range res.Rows {
			cells := make([]string, len(headers))
			for i := range headers {
				if i < len(row) && row[i] != nil {
					cells[i] = fmt.Sprintf("%v", row[i])
				}
			}
			t.AddRow(cells...)
		}
		p.PrintTable(t)

		// Rows go to stdout and everything about them to stderr, so the table pipes cleanly
		note := fmt.Sprintf("%d rows", res.RowCount)
		if res.RowCount == 1 {
			note = "1 row"
		}
		if res.Truncated {
			note += ", truncated at the row limit (raise it with --max-rows)"
		}
		fmt.Fprintf(os.Stderr, "\n%s · recorded as %s\n", note, res.QueryID)
		for _, ctx := range res.Context {
			line := fmt.Sprintf("  %s (%s)", ctx.Name, ctx.Type)
			if ctx.Description != "" {
				line += ": " + truncate(ctx.Description, 80)
			}
			if len(ctx.Tags) > 0 {
				line += " [" + strings.Join(ctx.Tags, ", ") + "]"
			}
			fmt.Fprintln(os.Stderr, line)
		}
		return nil
	},
}

var queryHistoryCmd = &cobra.Command{
	Use:   "history",
	Short: "List recorded queries, newest first",
	Long:  "Lists recorded queries. Without access:view you only see your own.",
	Example: `  marmot query history --decision denied
  marmot query history --mrn mrn://table/postgresql/orders -o json`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		opts := marmot.QueryHistoryOptions{}
		opts.Source, _ = cmd.Flags().GetString("source")
		opts.Decision, _ = cmd.Flags().GetString("decision")
		opts.Status, _ = cmd.Flags().GetString("status")
		opts.MRN, _ = cmd.Flags().GetString("mrn")
		opts.Limit, _ = cmd.Flags().GetInt64("limit")
		opts.Offset, _ = cmd.Flags().GetInt64("offset")
		c, err := newClient()
		if err != nil {
			return err
		}
		list, err := c.Query.History(cmd.Context(), opts)
		if err != nil {
			return err
		}
		return printOr(list, func(t *output.Table) {
			for _, q := range list.Queries {
				outcome := q.Status
				if q.Decision == "denied" {
					outcome = "denied"
				}
				t.AddRow(q.ID, q.StartedAt, q.AuditSubject, q.SourceName, outcome, strconv.FormatInt(q.RowsReturned, 10), truncate(oneLine(q.Statement), 50))
			}
			t.SetFooter("Showing %d of %d queries", len(list.Queries), list.Total)
		}, "ID", "STARTED", "WHO", "SOURCE", "OUTCOME", "ROWS", "STATEMENT")
	},
}

var queryShowCmd = &cobra.Command{
	Use:   "show <query-id>",
	Short: "Show one recorded query, including the catalog context when it ran",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := newClient()
		if err != nil {
			return err
		}
		q, err := c.Query.Get(cmd.Context(), args[0])
		if err != nil {
			return err
		}
		return printOr(q, func(t *output.Table) {
			t.AddRow("ID", q.ID)
			t.AddRow("Who", q.AuditSubject+" via "+q.Client)
			t.AddRow("Source", q.SourceName)
			t.AddRow("Decision", q.Decision)
			if reason, ok := q.DecisionDetail["reason"].(string); ok {
				t.AddRow("Reason", reason)
			}
			t.AddRow("Status", q.Status)
			t.AddRow("Rows", strconv.FormatInt(q.RowsReturned, 10))
			if q.Error != "" {
				t.AddRow("Error", q.Error)
			}
			t.AddRow("Started", q.StartedAt)
			t.AddRow("Tables", strings.Join(q.ReferencedMrns, ", "))
			t.AddRow("Statement", oneLine(q.Statement))
		}, "FIELD", "VALUE")
	},
}

var queryStatsCmd = &cobra.Command{
	Use:   "stats",
	Short: "Summarise query activity",
	Example: `  marmot query stats
  marmot query stats --since 7d -o json`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		since, _ := cmd.Flags().GetString("since")
		from, err := parseSince(since)
		if err != nil {
			return err
		}
		c, err := newClient()
		if err != nil {
			return err
		}
		stats, err := c.Query.Stats(cmd.Context(), from, "")
		if err != nil {
			return err
		}
		return printOr(stats, func(t *output.Table) {
			if stats.Totals != nil {
				t.AddRow("Queries", strconv.FormatInt(stats.Totals.Queries, 10))
				t.AddRow("Allowed", strconv.FormatInt(stats.Totals.Allowed, 10))
				t.AddRow("Denied", strconv.FormatInt(stats.Totals.Denied, 10))
				t.AddRow("Failed at the source", strconv.FormatInt(stats.Totals.Failed, 10))
				t.AddRow("Callers", strconv.FormatInt(stats.Totals.Callers, 10))
				t.AddRow("Tables read", strconv.FormatInt(stats.Totals.Tables, 10))
			}
			for _, g := range stats.TopTables {
				t.AddRow("Busiest table", fmt.Sprintf("%s (%d)", firstNonEmpty(g.Label, g.Key), g.Total))
			}
			for _, g := range stats.TopCallers {
				t.AddRow("Busiest caller", fmt.Sprintf("%s (%d allowed, %d denied)", g.Label, g.Allowed, g.Denied))
			}
			t.SetFooter("From %s to %s", stats.From, stats.To)
		}, "METRIC", "VALUE")
	},
}

func init() {
	querySourcesCmd.Flags().String("search", "", "Filter by source name or plugin")
	querySourcesCmd.Flags().Int64("limit", 50, "Maximum sources to return (at most 200)")
	querySourcesCmd.Flags().Int64("offset", 0, "Sources to skip")

	queryTablesCmd.Flags().String("search", "", "Filter by table name")
	queryTablesCmd.Flags().Int64("limit", 25, "Maximum tables to return (at most 100)")

	queryRunCmd.Flags().StringP("file", "f", "", "Read the statement from a file")
	queryRunCmd.Flags().String("table", "", "Preview a table: runs its SELECT ... LIMIT 10 in the source's dialect")
	queryRunCmd.Flags().Int64("max-rows", 0, "Row limit for this query (the server caps it)")

	queryHistoryCmd.Flags().String("source", "", "Only queries against this source")
	queryHistoryCmd.Flags().String("decision", "", "allowed or denied")
	queryHistoryCmd.Flags().String("status", "", "running, completed, failed or denied")
	queryHistoryCmd.Flags().String("mrn", "", "Only queries that read this table MRN")
	queryHistoryCmd.Flags().Int64("limit", 50, "Maximum queries to return (at most 500)")
	queryHistoryCmd.Flags().Int64("offset", 0, "Queries to skip")

	queryStatsCmd.Flags().String("since", "24h", "Window start: a duration such as 30m, 24h or 7d, or an RFC 3339 time")

	queryCmd.AddCommand(querySourcesCmd, queryTablesCmd, queryRunCmd, queryHistoryCmd, queryShowCmd, queryStatsCmd)
	rootCmd.AddCommand(queryCmd)
}

// printOr prints v as JSON or YAML when asked, and otherwise fills a table.
func printOr(v any, fill func(*output.Table), headers ...string) error {
	p := getPrinter()
	if p.IsRaw() {
		data, err := marshalPayload(v)
		if err != nil {
			return err
		}
		return p.PrintRaw(data)
	}
	t := output.NewTable(headers...)
	fill(t)
	p.PrintTable(t)
	return nil
}

// readStatement takes the statement from the argument, a file, or stdin when
// the argument is "-".
func readStatement(args []string, file string) (string, error) {
	if file != "" {
		data, err := os.ReadFile(file)
		if err != nil {
			return "", fmt.Errorf("reading %s: %w", file, err)
		}
		return strings.TrimSpace(string(data)), nil
	}
	if len(args) < 2 {
		return "", nil
	}
	if args[1] == "-" {
		data, err := io.ReadAll(os.Stdin)
		if err != nil {
			return "", fmt.Errorf("reading stdin: %w", err)
		}
		return strings.TrimSpace(string(data)), nil
	}
	return strings.TrimSpace(args[1]), nil
}

// previewStatement finds a table's preview statement in the source's own
// dialect.
func previewStatement(cmd *cobra.Command, c *marmot.Client, source, table string) (string, error) {
	tables, err := c.Query.Tables(cmd.Context(), source, table, 50)
	if err != nil {
		return "", err
	}
	for _, t := range tables {
		if strings.EqualFold(t.Name, table) && t.SampleStatement != "" {
			return t.SampleStatement, nil
		}
	}
	return "", withExitCode(ExitNotFound, fmt.Errorf("%s has no catalogued table called %q; list them with: marmot query tables %s", source, table, source))
}

var relativeTime = regexp.MustCompile(`^(\d+)\s*(m|min|h|d|w)$`)

// parseSince turns "30m", "24h", "7d", "2w" or an RFC 3339 time into an RFC
// 3339 time in the past.
func parseSince(value string) (string, error) {
	d, ok, err := relativeDuration(value)
	if err != nil {
		return "", err
	}
	if ok {
		return time.Now().UTC().Add(-d).Format(time.RFC3339), nil
	}
	return value, nil
}

func relativeDuration(value string) (time.Duration, bool, error) {
	value = strings.TrimSpace(strings.ToLower(value))
	if value == "" {
		return 0, false, nil
	}
	if m := relativeTime.FindStringSubmatch(value); m != nil {
		n, _ := strconv.Atoi(m[1])
		unit := map[string]time.Duration{"m": time.Minute, "min": time.Minute, "h": time.Hour, "d": 24 * time.Hour, "w": 7 * 24 * time.Hour}[m[2]]
		return time.Duration(n) * unit, true, nil
	}
	if _, err := time.Parse(time.RFC3339, value); err != nil {
		return 0, false, usageError("%q is neither a duration such as 30m, 24h or 7d nor an RFC 3339 time", value)
	}
	return 0, false, nil
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n-1] + "…"
}

func oneLine(s string) string {
	return strings.Join(strings.Fields(s), " ")
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}
