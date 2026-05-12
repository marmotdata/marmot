package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var adminCmd = &cobra.Command{
	Use:   "admin",
	Short: "Administrative operations",
}

var adminReindexCmd = &cobra.Command{
	Use:   "reindex",
	Short: "Trigger a search reindex",
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := newClient()
		if err != nil {
			return err
		}
		resp, err := c.Admin.Reindex(cmd.Context())
		if err != nil {
			return err
		}

		fmt.Printf("Reindex started: %s\n", resp.Status)
		return nil
	},
}

var adminReindexStatusCmd = &cobra.Command{
	Use:   "reindex-status",
	Short: "Check search reindex status",
	RunE: func(cmd *cobra.Command, args []string) error {
		p := getPrinter()
		c, err := newClient()
		if err != nil {
			return err
		}

		status, err := c.Admin.ReindexStatus(cmd.Context())
		if err != nil {
			return err
		}

		if p.IsRaw() {
			return p.PrintJSON(status)
		}

		fmt.Printf("ES Configured: %v\n", status.EsConfigured)
		fmt.Printf("Running:       %v\n", status.Running)
		return nil
	},
}

func init() {
	adminCmd.AddCommand(adminReindexCmd)
	adminCmd.AddCommand(adminReindexStatusCmd)
	rootCmd.AddCommand(adminCmd)
}
