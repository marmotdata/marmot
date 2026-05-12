package cmd

import (
	"fmt"
	"strings"

	marmot "github.com/marmotdata/marmot/sdk/go"
	"github.com/marmotdata/marmot/internal/cmd/output"
	"github.com/spf13/cobra"
)

var runsCmd = &cobra.Command{
	Use:   "runs",
	Short: "View pipeline runs",
}

var runsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List pipeline runs",
	RunE: func(cmd *cobra.Command, args []string) error {
		limit, _ := cmd.Flags().GetInt("limit")
		offset, _ := cmd.Flags().GetInt("offset")
		pipelines, _ := cmd.Flags().GetStringSlice("pipelines")
		statuses, _ := cmd.Flags().GetStringSlice("statuses")

		p := getPrinter()
		c, err := newClient()
		if err != nil {
			return err
		}

		opts := marmot.RunsListOptions{Limit: int64(limit), Offset: int64(offset)}
		if len(pipelines) > 0 {
			opts.Pipelines = strings.Join(pipelines, ",")
		}
		if len(statuses) > 0 {
			opts.Statuses = strings.Join(statuses, ",")
		}

		resp, err := c.Runs.List(cmd.Context(), opts)
		if err != nil {
			return err
		}

		if p.IsRaw() {
			data, err := marshalPayload(resp)
			if err != nil {
				return err
			}
			return p.PrintRaw(data)
		}

		t := output.NewTable("ID", "PIPELINE", "SOURCE", "STATUS", "STARTED", "COMPLETED")
		for _, r := range resp.Runs {
			t.AddRow(
				r.RunID,
				r.PipelineName,
				r.SourceName,
				string(r.Status),
				r.StartedAt,
				r.CompletedAt,
			)
		}
		t.SetFooter("Showing %d of %d runs", len(resp.Runs), resp.Total)
		p.PrintTable(t)
		return nil
	},
}

var runsGetCmd = &cobra.Command{
	Use:   "get <id>",
	Short: "Get run details",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		p := getPrinter()
		c, err := newClient()
		if err != nil {
			return err
		}

		r, err := c.Runs.Get(cmd.Context(), args[0])
		if err != nil {
			return err
		}

		if p.IsRaw() {
			data, err := marshalPayload(r)
			if err != nil {
				return err
			}
			return p.PrintRaw(data)
		}

		t := output.NewTable("FIELD", "VALUE")
		t.AddRow("Run ID", r.RunID)
		t.AddRow("Pipeline", r.PipelineName)
		t.AddRow("Source", r.SourceName)
		t.AddRow("Status", string(r.Status))
		t.AddRow("Started", r.StartedAt)
		if r.CompletedAt != "" {
			t.AddRow("Completed", r.CompletedAt)
		}
		if r.ErrorMessage != "" {
			t.AddRow("Error", r.ErrorMessage)
		}
		if r.Summary != nil {
			t.AddRow("Assets Created", fmt.Sprintf("%d", r.Summary.AssetsCreated))
			t.AddRow("Assets Updated", fmt.Sprintf("%d", r.Summary.AssetsUpdated))
			t.AddRow("Assets Deleted", fmt.Sprintf("%d", r.Summary.AssetsDeleted))
			t.AddRow("Errors", fmt.Sprintf("%d", r.Summary.ErrorsCount))
		}
		t.AddRow("Created By", r.CreatedBy)
		p.PrintTable(t)
		return nil
	},
}

var runsEntitiesCmd = &cobra.Command{
	Use:   "entities <run-id>",
	Short: "List entities processed in a run",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		limit, _ := cmd.Flags().GetInt("limit")
		offset, _ := cmd.Flags().GetInt("offset")
		p := getPrinter()
		c, err := newClient()
		if err != nil {
			return err
		}

		resp, err := c.Runs.Entities(cmd.Context(), args[0], marmot.RunEntitiesOptions{Limit: int64(limit), Offset: int64(offset)})
		if err != nil {
			return err
		}

		if p.IsRaw() {
			data, err := marshalPayload(resp)
			if err != nil {
				return err
			}
			return p.PrintRaw(data)
		}

		t := output.NewTable("TYPE", "MRN", "NAME", "STATUS")
		for _, e := range resp.Entities {
			t.AddRow(e.EntityType, e.EntityMrn, e.EntityName, e.Status)
		}
		t.SetFooter("Showing %d of %d entities", len(resp.Entities), resp.Total)
		p.PrintTable(t)
		return nil
	},
}

func init() {
	runsListCmd.Flags().Int("limit", 20, "Maximum number of results")
	runsListCmd.Flags().Int("offset", 0, "Offset for pagination")
	runsListCmd.Flags().StringSlice("pipelines", nil, "Filter by pipeline names")
	runsListCmd.Flags().StringSlice("statuses", nil, "Filter by statuses: "+strings.Join([]string{"running", "completed", "failed", "cancelled"}, ", "))

	runsEntitiesCmd.Flags().Int("limit", 20, "Maximum number of results")
	runsEntitiesCmd.Flags().Int("offset", 0, "Offset for pagination")

	runsCmd.AddCommand(runsListCmd)
	runsCmd.AddCommand(runsGetCmd)
	runsCmd.AddCommand(runsEntitiesCmd)
	rootCmd.AddCommand(runsCmd)
}
