package cmd

import (
	"github.com/spf13/cobra"

	_ "github.com/marmotdata/marmot/internal/plugin/providers/airflow"
	_ "github.com/marmotdata/marmot/internal/plugin/providers/asyncapi"
	_ "github.com/marmotdata/marmot/internal/plugin/providers/bigquery"
	_ "github.com/marmotdata/marmot/internal/plugin/providers/clickhouse"
	_ "github.com/marmotdata/marmot/internal/plugin/providers/dbt"
	_ "github.com/marmotdata/marmot/internal/plugin/providers/kafka"
	_ "github.com/marmotdata/marmot/internal/plugin/providers/mongodb"
	_ "github.com/marmotdata/marmot/internal/plugin/providers/mysql"
	_ "github.com/marmotdata/marmot/internal/plugin/providers/openapi"
	_ "github.com/marmotdata/marmot/internal/plugin/providers/postgresql"
	_ "github.com/marmotdata/marmot/internal/plugin/providers/s3"
	_ "github.com/marmotdata/marmot/internal/plugin/providers/sns"
	_ "github.com/marmotdata/marmot/internal/plugin/providers/sqs"
)

var rootCmd = &cobra.Command{
	Use:   "marmot",
	Short: "Marmot is a simple to use Data Catalog.",
}

func Execute() error {
	return rootCmd.Execute()
}
