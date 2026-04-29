package cmd

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Manage Marmot CLI configuration",
}

var configInitCmd = &cobra.Command{
	Use:   "init",
	Short: "Interactive setup for Marmot CLI configuration",
	RunE: func(cmd *cobra.Command, args []string) error {
		reader := bufio.NewReader(os.Stdin)

		fmt.Printf("Marmot CLI Configuration\n\n")

		currentHost := viper.GetString("host")
		fmt.Printf("Host [%s]: ", currentHost)
		hostInput, _ := reader.ReadString('\n')
		hostInput = strings.TrimSpace(hostInput)
		if hostInput == "" {
			hostInput = currentHost
		}

		fmt.Print("API Key: ")
		keyInput, _ := reader.ReadString('\n')
		keyInput = strings.TrimSpace(keyInput)

		currentOutput := viper.GetString("output")
		fmt.Printf("Default output format (table/json/yaml) [%s]: ", currentOutput)
		outputInput, _ := reader.ReadString('\n')
		outputInput = strings.TrimSpace(outputInput)
		if outputInput == "" {
			outputInput = currentOutput
		}

		viper.Set("host", hostInput)
		if keyInput != "" {
			viper.Set("api_key", keyInput)
		}
		viper.Set("output", outputInput)

		if err := writeConfig(); err != nil {
			return err
		}

		fmt.Printf("\nConfiguration saved.\n")
		return nil
	},
}

var configSetCmd = &cobra.Command{
	Use:   "set <key> <value>",
	Short: "Set a configuration value",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		key := args[0]
		value := args[1]

		switch key {
		case "host", "api_key", "output", "current_context":
			viper.Set(key, value)
		default:
			return fmt.Errorf("unknown config key: %s (valid keys: host, api_key, output, current_context)", key)
		}

		if err := writeConfig(); err != nil {
			return err
		}

		fmt.Printf("Set %s = %s\n", key, value)
		return nil
	},
}

var configGetCmd = &cobra.Command{
	Use:   "get <key>",
	Short: "Get a configuration value",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		key := args[0]

		switch key {
		case "host", "api_key", "output", "current_context":
			fmt.Println(viper.GetString(key))
		default:
			return fmt.Errorf("unknown config key: %s (valid keys: host, api_key, output, current_context)", key)
		}

		return nil
	},
}

var configListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all configuration values",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Printf("host = %s\n", viper.GetString("host"))
		if viper.GetString("api_key") != "" {
			fmt.Printf("api_key = ****\n")
		} else {
			fmt.Printf("api_key = (not set)\n")
		}
		fmt.Printf("output = %s\n", viper.GetString("output"))
		if ctx := viper.GetString("current_context"); ctx != "" {
			fmt.Printf("current_context = %s\n", ctx)
		}
		return nil
	},
}

func writeConfig() error {
	dir, err := configDir()
	if err != nil {
		return err
	}

	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("creating config directory: %w", err)
	}

	configPath := filepath.Join(dir, "config.yaml")
	viper.SetConfigFile(configPath)
	if err := viper.WriteConfig(); err != nil {
		return viper.SafeWriteConfig()
	}
	return nil
}

func init() {
	configCmd.AddCommand(configInitCmd)
	configCmd.AddCommand(configSetCmd)
	configCmd.AddCommand(configGetCmd)
	configCmd.AddCommand(configListCmd)
	rootCmd.AddCommand(configCmd)
}
