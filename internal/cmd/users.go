package cmd

import (
	"fmt"
	"strings"

	"github.com/marmotdata/marmot/client/client/users"
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
		c := newSwaggerClient()

		params := users.NewGetUsersMeParams()
		resp, err := c.Users.GetUsersMe(params, nil)
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

		u := resp.Payload
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
		c := newSwaggerClient()

		params := users.NewGetUsersParams()
		params.SetLimit(int64Ptr(limit))
		params.SetOffset(int64Ptr(offset))

		resp, err := c.Users.GetUsers(params)
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

		t := output.NewTable("ID", "USERNAME", "NAME", "ACTIVE", "ROLES")
		for _, u := range resp.Payload.Users {
			roles := make([]string, len(u.Roles))
			for i, r := range u.Roles {
				roles[i] = r.Name
			}
			t.AddRow(u.ID, u.Username, u.Name, fmt.Sprintf("%v", u.Active), strings.Join(roles, ", "))
		}
		t.SetFooter("Showing %d of %d users", len(resp.Payload.Users), resp.Payload.Total)
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
		c := newSwaggerClient()

		params := users.NewGetUsersIDParams()
		params.SetID(args[0])

		resp, err := c.Users.GetUsersID(params)
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

		u := resp.Payload
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
