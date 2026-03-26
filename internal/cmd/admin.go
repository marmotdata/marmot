package cmd

import (
	"fmt"

	"github.com/marmotdata/marmot/client/client/admin"
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
		c := newSwaggerClient()
		resp, err := c.Admin.PostAdminSearchReindex(admin.NewPostAdminSearchReindexParams())
		if err != nil {
			return err
		}

		fmt.Printf("Reindex started: %s\n", resp.Payload.Status)
		return nil
	},
}

var adminReindexStatusCmd = &cobra.Command{
	Use:   "reindex-status",
	Short: "Check search reindex status",
	RunE: func(cmd *cobra.Command, args []string) error {
		p := getPrinter()
		c := newSwaggerClient()

		resp, err := c.Admin.GetAdminSearchReindex(admin.NewGetAdminSearchReindexParams())
		if err != nil {
			return err
		}

		if p.IsRaw() {
			return p.PrintJSON(resp.Payload)
		}

		status := resp.Payload
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
