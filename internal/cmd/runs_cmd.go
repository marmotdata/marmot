package cmd

import (
	"fmt"
	"strings"

	"github.com/marmotdata/marmot/client/client/runs"
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
		c := newSwaggerClient()

		params := runs.NewGetRunsParams()
		params.SetLimit(int64Ptr(limit))
		params.SetOffset(int64Ptr(offset))
		if len(pipelines) > 0 {
			joined := strings.Join(pipelines, ",")
			params.SetPipelines(&joined)
		}
		if len(statuses) > 0 {
			joined := strings.Join(statuses, ",")
			params.SetStatuses(&joined)
		}

		resp, err := c.Runs.GetRuns(params)
		if err != nil {
			return err
		}

		if p.IsRaw() {
			data, err := marshalPayload(resp.Payload)
			if err != nil {
				return err
			}
			return p.PrintRaw(data)
		}

		t := output.NewTable("ID", "PIPELINE", "SOURCE", "STATUS", "STARTED", "COMPLETED")
		for _, r := range resp.Payload.Runs {
			t.AddRow(
				r.RunID,
				r.PipelineName,
				r.SourceName,
				string(r.Status),
				r.StartedAt,
				r.CompletedAt,
			)
		}
		t.SetFooter("Showing %d of %d runs", len(resp.Payload.Runs), resp.Payload.Total)
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
		c := newSwaggerClient()

		params := runs.NewGetRunsIDParams()
		params.SetID(args[0])

		resp, err := c.Runs.GetRunsID(params)
		if err != nil {
			return err
		}

		if p.IsRaw() {
			data, err := marshalPayload(resp.Payload)
			if err != nil {
				return err
			}
			return p.PrintRaw(data)
		}

		r := resp.Payload
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
		c := newSwaggerClient()

		params := runs.NewGetRunsIDEntitiesParams()
		params.SetID(args[0])
		params.SetLimit(int64Ptr(limit))
		params.SetOffset(int64Ptr(offset))

		resp, err := c.Runs.GetRunsIDEntities(params)
		if err != nil {
			return err
		}

		if p.IsRaw() {
			data, err := marshalPayload(resp.Payload)
			if err != nil {
				return err
			}
			return p.PrintRaw(data)
		}

		t := output.NewTable("TYPE", "MRN", "NAME", "STATUS")
		for _, e := range resp.Payload.Entities {
			t.AddRow(e.EntityType, e.EntityMrn, e.EntityName, e.Status)
		}
		t.SetFooter("Showing %d of %d entities", len(resp.Payload.Entities), resp.Payload.Total)
		p.PrintTable(t)
		return nil
	},
}

func init() {
	// list
	runsListCmd.Flags().Int("limit", 20, "Maximum number of results")
	runsListCmd.Flags().Int("offset", 0, "Offset for pagination")
	runsListCmd.Flags().StringSlice("pipelines", nil, "Filter by pipeline names")
	runsListCmd.Flags().StringSlice("statuses", nil, "Filter by statuses: "+strings.Join([]string{"running", "completed", "failed", "cancelled"}, ", "))

	// entities
	runsEntitiesCmd.Flags().Int("limit", 20, "Maximum number of results")
	runsEntitiesCmd.Flags().Int("offset", 0, "Offset for pagination")

	runsCmd.AddCommand(runsListCmd)
	runsCmd.AddCommand(runsGetCmd)
	runsCmd.AddCommand(runsEntitiesCmd)
	rootCmd.AddCommand(runsCmd)
}
