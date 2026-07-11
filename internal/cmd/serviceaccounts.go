package cmd

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	marmot "github.com/marmotdata/marmot/sdk/go"
	"github.com/marmotdata/marmot/internal/cmd/output"
	"github.com/spf13/cobra"
)

var serviceAccountsCmd = &cobra.Command{
	Use:   "service-accounts",
	Short: "Manage service accounts",
}

var serviceAccountsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List service accounts",
	RunE: func(cmd *cobra.Command, args []string) error {
		p := getPrinter()
		c, err := newClient()
		if err != nil {
			return err
		}

		accounts, err := c.ServiceAccounts.List(cmd.Context())
		if err != nil {
			return err
		}

		if p.IsRaw() {
			data, err := marshalPayload(accounts)
			if err != nil {
				return err
			}
			return p.PrintRaw(data)
		}

		if len(accounts) == 0 {
			fmt.Println("No service accounts found.")
			return nil
		}

		t := output.NewTable("ID", "NAME", "ACTIVE", "ROLES", "CREATED")
		for _, sa := range accounts {
			roleNames := make([]string, len(sa.Roles))
			for i, r := range sa.Roles {
				roleNames[i] = r.Name
			}
			t.AddRow(sa.ID, sa.Name, fmt.Sprintf("%v", sa.Active), strings.Join(roleNames, ", "), sa.CreatedAt)
		}
		p.PrintTable(t)
		return nil
	},
}

var serviceAccountsGetCmd = &cobra.Command{
	Use:   "get <id>",
	Short: "Get service account details",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		p := getPrinter()
		c, err := newClient()
		if err != nil {
			return err
		}

		sa, err := c.ServiceAccounts.Get(cmd.Context(), args[0])
		if err != nil {
			return err
		}

		if p.IsRaw() {
			data, err := marshalPayload(sa)
			if err != nil {
				return err
			}
			return p.PrintRaw(data)
		}

		roleNames := make([]string, len(sa.Roles))
		for i, r := range sa.Roles {
			roleNames[i] = r.Name
		}

		t := output.NewTable("FIELD", "VALUE")
		t.AddRow("ID", sa.ID)
		t.AddRow("Name", sa.Name)
		t.AddRow("Description", sa.Description)
		t.AddRow("Active", fmt.Sprintf("%v", sa.Active))
		t.AddRow("Roles", strings.Join(roleNames, ", "))
		t.AddRow("Created", sa.CreatedAt)
		p.PrintTable(t)
		return nil
	},
}

var serviceAccountsCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a new service account",
	RunE: func(cmd *cobra.Command, args []string) error {
		name, _ := cmd.Flags().GetString("name")
		if name == "" {
			return fmt.Errorf("--name is required")
		}
		description, _ := cmd.Flags().GetString("description")
		rolesFlag, _ := cmd.Flags().GetString("roles")

		var roleIDs []string
		if rolesFlag != "" {
			for _, r := range strings.Split(rolesFlag, ",") {
				if r = strings.TrimSpace(r); r != "" {
					roleIDs = append(roleIDs, r)
				}
			}
		}

		p := getPrinter()
		c, err := newClient()
		if err != nil {
			return err
		}

		sa, err := c.ServiceAccounts.Create(cmd.Context(), marmot.CreateServiceAccountInput{
			Name:        name,
			Description: description,
			RoleIDs:     roleIDs,
		})
		if err != nil {
			return err
		}

		if p.IsRaw() {
			data, err := marshalPayload(sa)
			if err != nil {
				return err
			}
			return p.PrintRaw(data)
		}

		fmt.Printf("Service account created.\n\n")
		fmt.Printf("  ID:   %s\n", sa.ID)
		fmt.Printf("  Name: %s\n", sa.Name)
		return nil
	},
}

var serviceAccountsDeleteCmd = &cobra.Command{
	Use:   "delete <id>",
	Short: "Delete a service account",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		yes, _ := cmd.Flags().GetBool("yes")
		c, err := newClient()
		if err != nil {
			return err
		}

		if !yes {
			fmt.Printf("Are you sure you want to delete service account %s? (y/N): ", args[0])
			reader := bufio.NewReader(os.Stdin)
			response, _ := reader.ReadString('\n')
			response = strings.TrimSpace(strings.ToLower(response))
			if response != "y" && response != "yes" {
				fmt.Println("Cancelled.")
				return nil
			}
		}

		if err := c.ServiceAccounts.Delete(cmd.Context(), args[0]); err != nil {
			return err
		}

		fmt.Printf("Service account %s deleted.\n", args[0])
		return nil
	},
}

var saAPIKeysCmd = &cobra.Command{
	Use:   "apikeys",
	Short: "Manage service account API keys",
}

var saAPIKeysListCmd = &cobra.Command{
	Use:   "list",
	Short: "List API keys for a service account",
	RunE: func(cmd *cobra.Command, args []string) error {
		saID, _ := cmd.Flags().GetString("service-account")
		if saID == "" {
			return fmt.Errorf("--service-account is required")
		}

		p := getPrinter()
		c, err := newClient()
		if err != nil {
			return err
		}

		keys, err := c.ServiceAccounts.ListAPIKeys(cmd.Context(), saID)
		if err != nil {
			return err
		}

		if p.IsRaw() {
			data, err := marshalPayload(keys)
			if err != nil {
				return err
			}
			return p.PrintRaw(data)
		}

		if len(keys) == 0 {
			fmt.Println("No API keys found.")
			return nil
		}

		t := output.NewTable("ID", "NAME", "CREATED", "EXPIRES", "LAST USED")
		for _, k := range keys {
			expires := "never"
			if k.ExpiresAt != "" {
				expires = k.ExpiresAt
			}
			lastUsed := "never"
			if k.LastUsedAt != "" {
				lastUsed = k.LastUsedAt
			}
			t.AddRow(k.ID, k.Name, k.CreatedAt, expires, lastUsed)
		}
		p.PrintTable(t)
		return nil
	},
}

var saAPIKeysCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a new API key for a service account",
	RunE: func(cmd *cobra.Command, args []string) error {
		saID, _ := cmd.Flags().GetString("service-account")
		if saID == "" {
			return fmt.Errorf("--service-account is required")
		}
		name, _ := cmd.Flags().GetString("name")
		if name == "" {
			return fmt.Errorf("--name is required")
		}
		expiresInDays, _ := cmd.Flags().GetInt64("expires-in-days")

		c, err := newClient()
		if err != nil {
			return err
		}

		key, err := c.ServiceAccounts.CreateAPIKey(cmd.Context(), saID, marmot.CreateServiceAccountAPIKeyInput{
			Name:          name,
			ExpiresInDays: expiresInDays,
		})
		if err != nil {
			return err
		}

		fmt.Printf("API key created successfully.\n\n")
		fmt.Printf("  Name: %s\n", key.Name)
		fmt.Printf("  Key:  %s\n\n", key.Key)
		fmt.Printf("Save this key — it won't be shown again.\n")
		return nil
	},
}

var saAPIKeysDeleteCmd = &cobra.Command{
	Use:   "delete <key-id>",
	Short: "Delete an API key from a service account",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		saID, _ := cmd.Flags().GetString("service-account")
		if saID == "" {
			return fmt.Errorf("--service-account is required")
		}
		yes, _ := cmd.Flags().GetBool("yes")

		c, err := newClient()
		if err != nil {
			return err
		}

		if !yes {
			fmt.Printf("Are you sure you want to delete API key %s? (y/N): ", args[0])
			reader := bufio.NewReader(os.Stdin)
			response, _ := reader.ReadString('\n')
			response = strings.TrimSpace(strings.ToLower(response))
			if response != "y" && response != "yes" {
				fmt.Println("Cancelled.")
				return nil
			}
		}

		if err := c.ServiceAccounts.DeleteAPIKey(cmd.Context(), saID, args[0]); err != nil {
			return err
		}

		fmt.Printf("API key %s deleted.\n", args[0])
		return nil
	},
}

func init() {
	serviceAccountsCreateCmd.Flags().String("name", "", "Service account name (required)")
	serviceAccountsCreateCmd.Flags().String("description", "", "Service account description")
	serviceAccountsCreateCmd.Flags().String("roles", "", "Comma-separated role IDs to assign")

	serviceAccountsDeleteCmd.Flags().BoolP("yes", "y", false, "Skip confirmation prompt")

	saAPIKeysListCmd.Flags().String("service-account", "", "Service account ID (required)")
	saAPIKeysCreateCmd.Flags().String("service-account", "", "Service account ID (required)")
	saAPIKeysCreateCmd.Flags().String("name", "", "API key name (required)")
	saAPIKeysCreateCmd.Flags().Int64("expires-in-days", 0, "Expiry in days (0 = never expires)")
	saAPIKeysDeleteCmd.Flags().String("service-account", "", "Service account ID (required)")
	saAPIKeysDeleteCmd.Flags().BoolP("yes", "y", false, "Skip confirmation prompt")

	saAPIKeysCmd.AddCommand(saAPIKeysListCmd)
	saAPIKeysCmd.AddCommand(saAPIKeysCreateCmd)
	saAPIKeysCmd.AddCommand(saAPIKeysDeleteCmd)

	serviceAccountsCmd.AddCommand(serviceAccountsListCmd)
	serviceAccountsCmd.AddCommand(serviceAccountsGetCmd)
	serviceAccountsCmd.AddCommand(serviceAccountsCreateCmd)
	serviceAccountsCmd.AddCommand(serviceAccountsDeleteCmd)
	serviceAccountsCmd.AddCommand(saAPIKeysCmd)

	rootCmd.AddCommand(serviceAccountsCmd)
}
