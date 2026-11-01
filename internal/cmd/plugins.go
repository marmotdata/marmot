package cmd

import (
	"fmt"
	"strings"

	"github.com/marmotdata/marmot/internal/cmd/output"
	marmot "github.com/marmotdata/marmot/sdk/go"
	"github.com/spf13/cobra"
)

var pluginsCmd = &cobra.Command{
	Use:   "plugins",
	Short: "Manage core plugin versions",
}

var pluginsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List core plugins, their versions and available updates",
	RunE: func(cmd *cobra.Command, args []string) error {
		p := getPrinter()
		c, err := newClient()
		if err != nil {
			return err
		}

		status, err := c.Plugins.Versions(cmd.Context())
		if err != nil {
			return err
		}

		if p.IsRaw() {
			data, err := marshalPayload(status)
			if err != nil {
				return err
			}
			return p.PrintRaw(data)
		}

		fmt.Println(pluginsHeaderLine(status))
		printPluginsTable(p, status.Plugins)
		return nil
	},
}

var pluginsOutdatedCmd = &cobra.Command{
	Use:   "outdated",
	Short: "List plugins with an available update or an open security advisory",
	RunE: func(cmd *cobra.Command, args []string) error {
		p := getPrinter()
		c, err := newClient()
		if err != nil {
			return err
		}

		status, err := c.Plugins.Versions(cmd.Context())
		if err != nil {
			return err
		}

		var outdated []*marmot.PluginVersionStatus
		for _, ps := range status.Plugins {
			if ps.UpdateAvailable || len(ps.Advisories) > 0 {
				outdated = append(outdated, ps)
			}
		}

		if p.IsRaw() {
			data, err := marshalPayload(outdated)
			if err != nil {
				return err
			}
			return p.PrintRaw(data)
		}

		if len(outdated) == 0 {
			fmt.Println("Every plugin is up to date.")
			return nil
		}
		printPluginsTable(p, outdated)
		return nil
	},
}

var pluginsUpdateCmd = &cobra.Command{
	Use:   "update [id[@version]]",
	Short: "Move a plugin to a newer version",
	Long: `Move a plugin to a newer version from the plugin index.

With no version, moves to the latest version that supports this Marmot and has
no open advisory. Pass --all to update every plugin that has one.`,
	Args: func(cmd *cobra.Command, args []string) error {
		all, _ := cmd.Flags().GetBool("all")
		if all {
			return cobra.ExactArgs(0)(cmd, args)
		}
		return cobra.ExactArgs(1)(cmd, args)
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		all, _ := cmd.Flags().GetBool("all")
		c, err := newClient()
		if err != nil {
			return err
		}

		if all {
			return updateAllPlugins(cmd, c)
		}

		id, version, _ := strings.Cut(args[0], "@")
		if version == "" {
			status, err := c.Plugins.Versions(cmd.Context())
			if err != nil {
				return err
			}
			ps := findPluginStatus(status, id)
			if ps == nil {
				return withExitCode(ExitNotFound, fmt.Errorf("plugin %q not found", id))
			}
			if ps.Latest == "" {
				fmt.Printf("%s is already at the newest version available (%s).\n", id, ps.Version)
				return nil
			}
			version = ps.Latest
		}

		if err := c.Plugins.SetVersion(cmd.Context(), id, version); err != nil {
			return err
		}
		fmt.Printf("%s updated to %s.\n", id, version)
		return nil
	},
}

func updateAllPlugins(cmd *cobra.Command, c *marmot.Client) error {
	status, err := c.Plugins.Versions(cmd.Context())
	if err != nil {
		return err
	}

	var failed []string
	updated := 0
	for _, ps := range status.Plugins {
		if !ps.UpdateAvailable {
			continue
		}
		if err := c.Plugins.SetVersion(cmd.Context(), ps.ID, ps.Latest); err != nil {
			fmt.Printf("%s: failed to update to %s: %v\n", ps.ID, ps.Latest, err)
			failed = append(failed, ps.ID)
			continue
		}
		fmt.Printf("%s updated to %s.\n", ps.ID, ps.Latest)
		updated++
	}
	if updated == 0 && len(failed) == 0 {
		fmt.Println("Every plugin is up to date.")
	}
	if len(failed) > 0 {
		return fmt.Errorf("failed to update: %s", strings.Join(failed, ", "))
	}
	return nil
}

var pluginsPinCmd = &cobra.Command{
	Use:   "pin <id>",
	Short: "Pin a plugin at the version it runs, so auto-update leaves it alone",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := newClient()
		if err != nil {
			return err
		}
		if err := c.Plugins.Pin(cmd.Context(), args[0], true); err != nil {
			return err
		}
		fmt.Printf("%s pinned at its current version.\n", args[0])
		return nil
	},
}

var pluginsUnpinCmd = &cobra.Command{
	Use:   "unpin <id>",
	Short: "Unpin a plugin, so it follows the release and auto-update versions again",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := newClient()
		if err != nil {
			return err
		}
		if err := c.Plugins.Pin(cmd.Context(), args[0], false); err != nil {
			return err
		}
		fmt.Printf("%s unpinned.\n", args[0])
		return nil
	},
}

var pluginsRollbackCmd = &cobra.Command{
	Use:   "rollback <id>",
	Short: "Roll a plugin back to the version it ran before its last change, and pin it",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := newClient()
		if err != nil {
			return err
		}
		if err := c.Plugins.Rollback(cmd.Context(), args[0]); err != nil {
			return err
		}
		fmt.Printf("%s rolled back and pinned.\n", args[0])
		return nil
	},
}

var pluginsResetCmd = &cobra.Command{
	Use:   "reset <id>",
	Short: "Return a plugin to the version this Marmot release pins",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := newClient()
		if err != nil {
			return err
		}
		if err := c.Plugins.Reset(cmd.Context(), args[0]); err != nil {
			return err
		}
		fmt.Printf("%s reset to the release version.\n", args[0])
		return nil
	},
}

var pluginsCheckCmd = &cobra.Command{
	Use:   "check",
	Short: "Check the plugin index for new versions and advisories now",
	RunE: func(cmd *cobra.Command, args []string) error {
		p := getPrinter()
		c, err := newClient()
		if err != nil {
			return err
		}

		updated, err := c.Plugins.Check(cmd.Context())
		if err != nil {
			return err
		}

		if p.IsRaw() {
			return p.PrintJSON(struct {
				Updated bool `json:"updated"`
			}{updated})
		}

		if updated {
			fmt.Println("A newer plugin index was found and is now in use.")
		} else {
			fmt.Println("No newer plugin index was found.")
		}
		return nil
	},
}

// printPluginsTable renders plugin version statuses as a table of ID, VERSION, ORIGIN, PINNED, LATEST and STATUS.
func printPluginsTable(p *output.Printer, plugins []*marmot.PluginVersionStatus) {
	t := output.NewTable("ID", "VERSION", "ORIGIN", "PINNED", "LATEST", "STATUS")
	for _, ps := range plugins {
		t.AddRow(ps.ID, dashIfEmpty(ps.Version), ps.Origin, fmt.Sprintf("%v", ps.Pinned), dashIfEmpty(ps.Latest), pluginStatusLine(ps))
	}
	p.PrintTable(t)
}

// pluginStatusLine summarises a plugin's status for the STATUS column: an open advisory wins over a
// plain update, since moving off the advisory is the more urgent action.
func pluginStatusLine(ps *marmot.PluginVersionStatus) string {
	if len(ps.Advisories) > 0 {
		ids := make([]string, len(ps.Advisories))
		for i, a := range ps.Advisories {
			ids[i] = a.ID
		}
		return "security: " + strings.Join(ids, ", ")
	}
	if ps.UpdateAvailable {
		return "update available"
	}
	if ps.Error != "" {
		return "error: " + ps.Error
	}
	return "ok"
}

// pluginsHeaderLine summarises the auto-update policy and plugin index state shown above the plugins table.
func pluginsHeaderLine(st *marmot.PluginUpdatesStatus) string {
	autoUpdate := st.AutoUpdate
	if autoUpdate == "" {
		autoUpdate = "security"
	}
	if !st.Enabled {
		return fmt.Sprintf("Auto-update: %s  |  Plugin index: disabled\n", autoUpdate)
	}

	checked := "never"
	if st.Index != nil && st.Index.CheckedAt != "" {
		checked = st.Index.CheckedAt
	}
	line := fmt.Sprintf("Auto-update: %s  |  Plugin index last checked: %s", autoUpdate, checked)
	if st.Index != nil && st.Index.Error != "" {
		line += fmt.Sprintf("  (error: %s)", st.Index.Error)
	}
	return line + "\n"
}

// findPluginStatus returns the status for the plugin with the given ID, or nil if it is not a core plugin.
func findPluginStatus(st *marmot.PluginUpdatesStatus, id string) *marmot.PluginVersionStatus {
	for _, ps := range st.Plugins {
		if ps.ID == id {
			return ps
		}
	}
	return nil
}

// dashIfEmpty returns "-" for an empty string, so empty table cells are not blank.
func dashIfEmpty(s string) string {
	if s == "" {
		return "-"
	}
	return s
}

func init() {
	pluginsUpdateCmd.Flags().Bool("all", false, "Update every plugin that has an available update")

	pluginsCmd.AddCommand(pluginsListCmd)
	pluginsCmd.AddCommand(pluginsOutdatedCmd)
	pluginsCmd.AddCommand(pluginsUpdateCmd)
	pluginsCmd.AddCommand(pluginsPinCmd)
	pluginsCmd.AddCommand(pluginsUnpinCmd)
	pluginsCmd.AddCommand(pluginsRollbackCmd)
	pluginsCmd.AddCommand(pluginsResetCmd)
	pluginsCmd.AddCommand(pluginsCheckCmd)
	rootCmd.AddCommand(pluginsCmd)
}
