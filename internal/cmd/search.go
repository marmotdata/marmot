package cmd

import (
	"strings"

	marmot "github.com/marmotdata/marmot/sdk/go"
	"github.com/marmotdata/marmot/internal/cmd/output"
	"github.com/spf13/cobra"
)

var searchCmd = &cobra.Command{
	Use:   "search <query>",
	Short: "Search across assets, glossary, teams, and users",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		limit, _ := cmd.Flags().GetInt("limit")
		offset, _ := cmd.Flags().GetInt("offset")
		types, _ := cmd.Flags().GetStringSlice("types")

		p := getPrinter()
		c, err := newClient()
		if err != nil {
			return err
		}

		results, err := c.Search.Query(cmd.Context(), args[0], marmot.SearchOptions{
			Types:  types,
			Limit:  int64(limit),
			Offset: int64(offset),
		})
		if err != nil {
			return err
		}

		if p.IsRaw() {
			data, err := marshalPayload(results)
			if err != nil {
				return err
			}
			return p.PrintRaw(data)
		}

		t := output.NewTable("ID", "NAME", "TYPE", "DESCRIPTION")
		for _, r := range results.Results {
			desc := r.Description
			if len(desc) > 60 {
				desc = desc[:57] + "..."
			}
			t.AddRow(r.ID, r.Name, string(r.Type), desc)
		}
		t.SetFooter("Showing %d of %d results", len(results.Results), results.Total)
		p.PrintTable(t)
		return nil
	},
}

func init() {
	searchCmd.Flags().Int("limit", 20, "Maximum number of results")
	searchCmd.Flags().Int("offset", 0, "Offset for pagination")
	searchCmd.Flags().StringSlice("types", nil, "Filter by types: "+strings.Join([]string{"asset", "glossary", "team", "user"}, ", "))
	rootCmd.AddCommand(searchCmd)
}
