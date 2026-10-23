package cmd

import (
	"fmt"

	"github.com/marmotdata/marmot/internal/cmd/output"
	marmot "github.com/marmotdata/marmot/sdk/go"
	"github.com/spf13/cobra"
)

var apikeysCmd = &cobra.Command{
	Use:   "apikeys",
	Short: "Manage API keys",
}

var apikeysListCmd = &cobra.Command{
	Use:   "list",
	Short: "List your API keys",
	RunE: func(cmd *cobra.Command, args []string) error {
		p := getPrinter()
		c, err := newClient()
		if err != nil {
			return err
		}

		keys, err := c.APIKeys.List(cmd.Context())
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

		t := output.NewTable("ID", "NAME", "CREATED", "LAST USED")
		for _, k := range keys {
			lastUsed := "never"
			if k.LastUsedAt != "" {
				lastUsed = k.LastUsedAt
			}
			t.AddRow(k.ID, k.Name, k.CreatedAt, lastUsed)
		}
		p.PrintTable(t)
		return nil
	},
}

var apikeysCreateCmd = &cobra.Command{
	Use:   "create <name>",
	Short: "Create a new API key",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := newClient()
		if err != nil {
			return err
		}

		key, err := c.APIKeys.Create(cmd.Context(), marmot.CreateAPIKeyInput{Name: args[0]})
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

var apikeysDeleteCmd = &cobra.Command{
	Use:   "delete <id>",
	Short: "Delete an API key",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := confirm(cmd, fmt.Sprintf("Delete API key %s?", args[0])); err != nil {
			return err
		}
		c, err := newClient()
		if err != nil {
			return err
		}

		if err := c.APIKeys.Delete(cmd.Context(), args[0]); err != nil {
			return err
		}

		fmt.Printf("API key %s deleted.\n", args[0])
		return nil
	},
}

func init() {
	apikeysDeleteCmd.Flags().BoolP("yes", "y", false, "Skip confirmation prompt")

	apikeysCmd.AddCommand(apikeysListCmd)
	apikeysCmd.AddCommand(apikeysCreateCmd)
	apikeysCmd.AddCommand(apikeysDeleteCmd)
	rootCmd.AddCommand(apikeysCmd)
}
