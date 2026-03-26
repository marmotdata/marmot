package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/marmotdata/marmot/internal/cmd/output"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	_ "github.com/marmotdata/marmot/internal/plugin/providers/airflow"
	_ "github.com/marmotdata/marmot/internal/plugin/providers/asyncapi"
	_ "github.com/marmotdata/marmot/internal/plugin/providers/azureblob"
	_ "github.com/marmotdata/marmot/internal/plugin/providers/bigquery"
	_ "github.com/marmotdata/marmot/internal/plugin/providers/clickhouse"
	_ "github.com/marmotdata/marmot/internal/plugin/providers/dbt"
	_ "github.com/marmotdata/marmot/internal/plugin/providers/gcs"
	_ "github.com/marmotdata/marmot/internal/plugin/providers/kafka"
	_ "github.com/marmotdata/marmot/internal/plugin/providers/mongodb"
	_ "github.com/marmotdata/marmot/internal/plugin/providers/mysql"
	_ "github.com/marmotdata/marmot/internal/plugin/providers/openapi"
	_ "github.com/marmotdata/marmot/internal/plugin/providers/postgresql"
	_ "github.com/marmotdata/marmot/internal/plugin/providers/s3"
	_ "github.com/marmotdata/marmot/internal/plugin/providers/sns"
	_ "github.com/marmotdata/marmot/internal/plugin/providers/sqs"
)

var (
	globalHost   string
	globalAPIKey string
	globalOutput string
)

var rootCmd = &cobra.Command{
	Use:   "marmot",
	Short: "Marmot is a simple to use Data Catalog.",
}

func init() {
	cobra.OnInitialize(initConfig)

	rootCmd.PersistentFlags().StringVar(&globalHost, "host", "", "Marmot API host (default: http://localhost:8080)")
	rootCmd.PersistentFlags().StringVar(&globalAPIKey, "api-key", "", "API key for authentication")
	rootCmd.PersistentFlags().StringVarP(&globalOutput, "output", "o", "", "Output format: table, json, yaml (default: table)")

	viper.BindPFlag("host", rootCmd.PersistentFlags().Lookup("host"))
	viper.BindPFlag("api_key", rootCmd.PersistentFlags().Lookup("api-key"))
	viper.BindPFlag("output", rootCmd.PersistentFlags().Lookup("output"))

	viper.SetDefault("host", "http://localhost:8080")
	viper.SetDefault("output", "table")
}

func initConfig() {
	viper.SetEnvPrefix("MARMOT")
	viper.AutomaticEnv()

	// Map env var names: MARMOT_HOST, MARMOT_API_KEY, MARMOT_OUTPUT
	viper.BindEnv("host", "MARMOT_HOST")
	viper.BindEnv("api_key", "MARMOT_API_KEY")
	viper.BindEnv("output", "MARMOT_OUTPUT")

	// Config file: ~/.config/marmot/config.yaml
	configDir, err := os.UserConfigDir()
	if err == nil {
		configPath := filepath.Join(configDir, "marmot")
		viper.AddConfigPath(configPath)
		viper.SetConfigName("config")
		viper.SetConfigType("yaml")
		viper.ReadInConfig() // Ignore error — config file is optional
	}
}

// getPrinter creates a Printer based on resolved config.
func getPrinter() *output.Printer {
	return output.NewPrinter(viper.GetString("output"), os.Stdout)
}

// getHost returns the resolved host for commands that need it directly.
func getHost() string {
	return viper.GetString("host")
}

// getAPIKey returns the resolved API key for commands that need it directly.
func getAPIKey() string {
	return viper.GetString("api_key")
}

// configDir returns the marmot config directory path.
func configDir() (string, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("cannot determine config directory: %w", err)
	}
	return filepath.Join(base, "marmot"), nil
}

func Execute() error {
	return rootCmd.Execute()
}
