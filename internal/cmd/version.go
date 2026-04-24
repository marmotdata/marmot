package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var Version = "dev"

func init() {
	rootCmd.AddCommand(versionCmd)
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print Marmot version information",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Printf("marmot v%s\n", Version)

		return nil
	},
}
