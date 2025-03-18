package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var (
	ClientVersion = "0.1.0"
	ServerVersion = "0.1.0"
)

func init() {
	rootCmd.AddCommand(versionCmd)
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print Marmot version information",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Printf("Client v%s\n", ClientVersion)
		fmt.Printf("Server v%s\n", ServerVersion)

		return nil
	},
}
