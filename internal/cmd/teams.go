package cmd

import (
	"fmt"

	"github.com/marmotdata/marmot/client/client/teams"
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
		c := newSwaggerClient()

		params := teams.NewGetTeamsParams()
		params.SetLimit(int64Ptr(limit))
		params.SetOffset(int64Ptr(offset))

		resp, err := c.Teams.GetTeams(params)
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

		t := output.NewTable("ID", "NAME", "DESCRIPTION")
		for _, team := range resp.Payload.Teams {
			desc := team.Description
			if len(desc) > 60 {
				desc = desc[:57] + "..."
			}
			t.AddRow(team.ID, team.Name, desc)
		}
		t.SetFooter("Showing %d of %d teams", len(resp.Payload.Teams), resp.Payload.Total)
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
		c := newSwaggerClient()

		params := teams.NewGetTeamsIDParams()
		params.SetID(args[0])

		resp, err := c.Teams.GetTeamsID(params)
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

		team := resp.Payload
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
		c := newSwaggerClient()

		params := teams.NewGetTeamsIDMembersParams()
		params.SetID(args[0])

		resp, err := c.Teams.GetTeamsIDMembers(params)
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

		if len(resp.Payload.Members) == 0 {
			fmt.Println("No members found.")
			return nil
		}

		t := output.NewTable("USER ID", "USERNAME", "NAME", "ROLE", "JOINED")
		for _, m := range resp.Payload.Members {
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
