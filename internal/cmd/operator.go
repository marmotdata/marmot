package cmd

import "github.com/marmotdata/marmot/internal/operator"

func init() {
	rootCmd.AddCommand(operator.NewCommand())
}
