package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// ContextEntry represents a saved server context.
type ContextEntry struct {
	Host string `yaml:"host" mapstructure:"host"`
}

var contextCmd = &cobra.Command{
	Use:   "context",
	Short: "Manage Marmot server contexts",
}

var contextListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all contexts",
	RunE: func(cmd *cobra.Command, args []string) error {
		contexts := getContexts()
		current := currentContextName()

		if len(contexts) == 0 {
			fmt.Println("No contexts configured. Run 'marmot login' to create one.")
			return nil
		}

		for name, ctx := range contexts {
			marker := " "
			if name == current {
				marker = "*"
			}

			status := "(no token)"
			if _, ok := getCachedToken(name); ok {
				status = "(token valid)"
			} else {
				store, err := loadCredentials()
				if err == nil {
					if _, exists := store.Tokens[name]; exists {
						status = "(token expired)"
					}
				}
			}

			fmt.Printf("%s %-25s %s  %s\n", marker, name, ctx.Host, status)
		}
		return nil
	},
}

var contextUseCmd = &cobra.Command{
	Use:   "use <name>",
	Short: "Switch to a different context",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]
		contexts := getContexts()

		if _, ok := contexts[name]; !ok {
			return fmt.Errorf("context %q not found", name)
		}

		viper.Set("current_context", name)
		if err := writeConfig(); err != nil {
			return err
		}

		fmt.Printf("Switched to context %q\n", name)
		return nil
	},
}

var contextDeleteCmd = &cobra.Command{
	Use:   "delete <name>",
	Short: "Delete a context and its cached token",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]
		contexts := getContexts()

		if _, ok := contexts[name]; !ok {
			return fmt.Errorf("context %q not found", name)
		}

		delete(contexts, name)
		viper.Set("contexts", contexts)

		if currentContextName() == name {
			viper.Set("current_context", "")
		}

		if err := writeConfig(); err != nil {
			return err
		}

		_ = deleteCachedToken(name)

		fmt.Printf("Deleted context %q\n", name)
		return nil
	},
}

// currentContextName returns the active context name from config.
func currentContextName() string {
	return viper.GetString("current_context")
}

// getActiveContext returns the current context name and entry, or empty if none.
func getActiveContext() (string, *ContextEntry) {
	name := currentContextName()
	if name == "" {
		return "", nil
	}

	contexts := getContexts()
	ctx, ok := contexts[name]
	if !ok {
		return "", nil
	}

	return name, &ctx
}

// getContexts returns all configured contexts.
func getContexts() map[string]ContextEntry {
	raw := viper.GetStringMap("contexts")
	result := make(map[string]ContextEntry)

	for name, v := range raw {
		if entry, ok := v.(map[string]interface{}); ok {
			host, _ := entry["host"].(string)
			result[name] = ContextEntry{Host: host}
		}
	}

	return result
}

// setContext adds or updates a context in config and writes it.
func setContext(name string, ctx ContextEntry) error {
	contexts := getContexts()
	contexts[name] = ctx

	raw := make(map[string]interface{})
	for k, v := range contexts {
		raw[k] = map[string]interface{}{"host": v.Host}
	}
	viper.Set("contexts", raw)
	viper.Set("current_context", name)

	return writeConfig()
}

// resolveHost returns the host to use, checking --host flag, active context, and legacy config.
func resolveHost() string {
	if globalHost != "" {
		return globalHost
	}
	if _, ctx := getActiveContext(); ctx != nil {
		return ctx.Host
	}
	return viper.GetString("host")
}

func init() {
	contextCmd.AddCommand(contextListCmd)
	contextCmd.AddCommand(contextUseCmd)
	contextCmd.AddCommand(contextDeleteCmd)
	rootCmd.AddCommand(contextCmd)
}
