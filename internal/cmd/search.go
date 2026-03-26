package cmd

import (
	"strings"

	"github.com/marmotdata/marmot/client/client/search"
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
		c := newSwaggerClient()

		params := search.NewGetSearchParams()
		params.SetQ(args[0])
		params.SetLimit(int64Ptr(limit))
		params.SetOffset(int64Ptr(offset))
		if len(types) > 0 {
			params.SetTypes(types)
		}

		resp, err := c.Search.GetSearch(params)
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

		t := output.NewTable("ID", "NAME", "TYPE", "DESCRIPTION")
		for _, r := range resp.Payload.Results {
			desc := r.Description
			if len(desc) > 60 {
				desc = desc[:57] + "..."
			}
			t.AddRow(r.ID, r.Name, string(r.Type), desc)
		}
		t.SetFooter("Showing %d of %d results", len(resp.Payload.Results), resp.Payload.Total)
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
