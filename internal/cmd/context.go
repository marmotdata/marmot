package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// ContextEntry represents a saved server context.
type ContextEntry struct {
	Host string `json:"host"`
}

// ContextStore holds all saved contexts, keyed by context name. Contexts live
// beside the config rather than in it: names are hostnames, and Viper is
// dot-delimited, so writing them through Viper splits a name into nested keys.
type ContextStore struct {
	Contexts map[string]ContextEntry `json:"contexts"`
}

var contextCmd = &cobra.Command{
	Use:   "context",
	Short: "Manage Marmot server contexts",
}

var contextListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all contexts",
	RunE: func(cmd *cobra.Command, args []string) error {
		contexts, err := loadContexts()
		if err != nil {
			return err
		}
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
		contexts, err := loadContexts()
		if err != nil {
			return err
		}

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
		contexts, err := loadContexts()
		if err != nil {
			return err
		}

		if _, ok := contexts[name]; !ok {
			return fmt.Errorf("context %q not found", name)
		}

		delete(contexts, name)
		if err := saveContexts(contexts); err != nil {
			return err
		}

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

	contexts, err := loadContexts()
	if err != nil {
		return "", nil
	}
	ctx, ok := contexts[name]
	if !ok {
		return "", nil
	}

	return name, &ctx
}

func contextsPath() (string, error) {
	dir, err := configDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "contexts.json"), nil
}

// loadContexts returns all saved contexts.
func loadContexts() (map[string]ContextEntry, error) {
	p, err := contextsPath()
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(p)
	if err != nil {
		if os.IsNotExist(err) {
			return make(map[string]ContextEntry), nil
		}
		return nil, err
	}

	var store ContextStore
	if err := json.Unmarshal(data, &store); err != nil {
		return nil, fmt.Errorf("parsing %s: %w", p, err)
	}
	if store.Contexts == nil {
		store.Contexts = make(map[string]ContextEntry)
	}
	return store.Contexts, nil
}

func saveContexts(contexts map[string]ContextEntry) error {
	p, err := contextsPath()
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
		return err
	}

	data, err := json.MarshalIndent(ContextStore{Contexts: contexts}, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(p, data, 0o600)
}

// setContext adds or updates a context and makes it current.
func setContext(name string, ctx ContextEntry) error {
	contexts, err := loadContexts()
	if err != nil {
		return err
	}
	contexts[name] = ctx

	if err := saveContexts(contexts); err != nil {
		return err
	}

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
