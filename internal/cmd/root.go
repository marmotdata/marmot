package cmd

import (
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "marmot",
	Short: "Marmot is a simple to use Data Catalog.",
}

func Execute() error {
	return rootCmd.Execute()
}
