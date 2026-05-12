package cmd

import (
	"fmt"
	"strings"

	marmot "github.com/marmotdata/marmot/sdk/go"
	"github.com/marmotdata/marmot/internal/cmd/output"
	"github.com/spf13/cobra"
)

var usersCmd = &cobra.Command{
	Use:   "users",
	Short: "View users",
}

var usersMeCmd = &cobra.Command{
	Use:   "me",
	Short: "Show the currently authenticated user",
	RunE: func(cmd *cobra.Command, args []string) error {
		p := getPrinter()
		c, err := newClient()
		if err != nil {
			return err
		}

		u, err := c.Users.Me(cmd.Context())
		if err != nil {
			return err
		}

		if p.IsRaw() {
			data, err := marshalPayload(u)
			if err != nil {
				return err
			}
			return p.PrintRaw(data)
		}

		t := output.NewTable("FIELD", "VALUE")
		t.AddRow("ID", u.ID)
		t.AddRow("Username", u.Username)
		t.AddRow("Name", u.Name)
		t.AddRow("Active", fmt.Sprintf("%v", u.Active))
		roles := make([]string, len(u.Roles))
		for i, r := range u.Roles {
			roles[i] = r.Name
		}
		t.AddRow("Roles", strings.Join(roles, ", "))
		t.AddRow("Created", u.CreatedAt)
		p.PrintTable(t)
		return nil
	},
}

var usersListCmd = &cobra.Command{
	Use:   "list",
	Short: "List users",
	RunE: func(cmd *cobra.Command, args []string) error {
		limit, _ := cmd.Flags().GetInt("limit")
		offset, _ := cmd.Flags().GetInt("offset")
		p := getPrinter()
		c, err := newClient()
		if err != nil {
			return err
		}

		resp, err := c.Users.List(cmd.Context(), marmot.UsersListOptions{Limit: int64(limit), Offset: int64(offset)})
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

		t := output.NewTable("ID", "USERNAME", "NAME", "ACTIVE", "ROLES")
		for _, u := range resp.Users {
			roles := make([]string, len(u.Roles))
			for i, r := range u.Roles {
				roles[i] = r.Name
			}
			t.AddRow(u.ID, u.Username, u.Name, fmt.Sprintf("%v", u.Active), strings.Join(roles, ", "))
		}
		t.SetFooter("Showing %d of %d users", len(resp.Users), resp.Total)
		p.PrintTable(t)
		return nil
	},
}

var usersGetCmd = &cobra.Command{
	Use:   "get <id>",
	Short: "Get user details",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		p := getPrinter()
		c, err := newClient()
		if err != nil {
			return err
		}

		u, err := c.Users.Get(cmd.Context(), args[0])
		if err != nil {
			return err
		}

		if p.IsRaw() {
			data, err := marshalPayload(u)
			if err != nil {
				return err
			}
			return p.PrintRaw(data)
		}

		t := output.NewTable("FIELD", "VALUE")
		t.AddRow("ID", u.ID)
		t.AddRow("Username", u.Username)
		t.AddRow("Name", u.Name)
		t.AddRow("Active", fmt.Sprintf("%v", u.Active))
		roles := make([]string, len(u.Roles))
		for i, r := range u.Roles {
			roles[i] = r.Name
		}
		t.AddRow("Roles", strings.Join(roles, ", "))
		t.AddRow("Created", u.CreatedAt)
		p.PrintTable(t)
		return nil
	},
}

func init() {
	usersListCmd.Flags().Int("limit", 20, "Maximum number of results")
	usersListCmd.Flags().Int("offset", 0, "Offset for pagination")

	usersCmd.AddCommand(usersMeCmd)
	usersCmd.AddCommand(usersListCmd)
	usersCmd.AddCommand(usersGetCmd)
	rootCmd.AddCommand(usersCmd)
}
