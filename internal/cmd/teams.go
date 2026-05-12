package cmd

import (
	"fmt"

	marmot "github.com/marmotdata/marmot/sdk/go"
	"github.com/marmotdata/marmot/internal/cmd/output"
	"github.com/spf13/cobra"
)

var teamsCmd = &cobra.Command{
	Use:   "teams",
	Short: "View teams",
}

var teamsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List teams",
	RunE: func(cmd *cobra.Command, args []string) error {
		limit, _ := cmd.Flags().GetInt("limit")
		offset, _ := cmd.Flags().GetInt("offset")
		p := getPrinter()
		c, err := newClient()
		if err != nil {
			return err
		}

		resp, err := c.Teams.List(cmd.Context(), marmot.TeamsListOptions{Limit: int64(limit), Offset: int64(offset)})
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

		t := output.NewTable("ID", "NAME", "DESCRIPTION")
		for _, team := range resp.Teams {
			desc := team.Description
			if len(desc) > 60 {
				desc = desc[:57] + "..."
			}
			t.AddRow(team.ID, team.Name, desc)
		}
		t.SetFooter("Showing %d of %d teams", len(resp.Teams), resp.Total)
		p.PrintTable(t)
		return nil
	},
}

var teamsGetCmd = &cobra.Command{
	Use:   "get <id>",
	Short: "Get team details",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		p := getPrinter()
		c, err := newClient()
		if err != nil {
			return err
		}

		team, err := c.Teams.Get(cmd.Context(), args[0])
		if err != nil {
			return err
		}

		if p.IsRaw() {
			data, err := marshalPayload(team)
			if err != nil {
				return err
			}
			return p.PrintRaw(data)
		}

		t := output.NewTable("FIELD", "VALUE")
		t.AddRow("ID", team.ID)
		t.AddRow("Name", team.Name)
		t.AddRow("Description", team.Description)
		t.AddRow("Created", team.CreatedAt)
		t.AddRow("Updated", team.UpdatedAt)
		p.PrintTable(t)
		return nil
	},
}

var teamsMembersCmd = &cobra.Command{
	Use:   "members <team-id>",
	Short: "List team members",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		p := getPrinter()
		c, err := newClient()
		if err != nil {
			return err
		}

		resp, err := c.Teams.Members(cmd.Context(), args[0])
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

		if len(resp.Members) == 0 {
			fmt.Println("No members found.")
			return nil
		}

		t := output.NewTable("USER ID", "USERNAME", "NAME", "ROLE", "JOINED")
		for _, m := range resp.Members {
			t.AddRow(m.UserID, m.Username, m.Name, m.Role, m.JoinedAt)
		}
		p.PrintTable(t)
		return nil
	},
}

func init() {
	teamsListCmd.Flags().Int("limit", 50, "Maximum number of results")
	teamsListCmd.Flags().Int("offset", 0, "Offset for pagination")

	teamsCmd.AddCommand(teamsListCmd)
	teamsCmd.AddCommand(teamsGetCmd)
	teamsCmd.AddCommand(teamsMembersCmd)
	rootCmd.AddCommand(teamsCmd)
}
