package cmd

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/marmotdata/marmot/internal/plugin/release"
)

var pluginCmd = &cobra.Command{
	Use:   "plugin",
	Short: "Manage Marmot plugins",
}

var (
	pluginPushSource    string
	pluginPushDist      string
	pluginPushRepoURL   string
	pluginPushPlainHTTP bool
)

var pluginPushCmd = &cobra.Command{
	Use:   "push <ref>",
	Short: "Publish a plugin OCI artefact to a registry",
	Long: `Publish a plugin to an OCI registry.

Packs the per-platform binaries under --dist under a multi-arch index
tagged with the ref, and attaches an info referrer with the plugin's
README, --dump-metadata output, and source-scanned asset schemas.

Build binaries first with GoReleaser (or equivalent). Auth is taken
from the standard Docker credentials store.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		result, err := release.Push(cmd.Context(), release.Options{
			Ref:       args[0],
			Source:    pluginPushSource,
			Dist:      pluginPushDist,
			RepoURL:   pluginPushRepoURL,
			PlainHTTP: pluginPushPlainHTTP,
		})
		if err != nil {
			return err
		}
		fmt.Fprintf(cmd.OutOrStdout(), "index: %s\ninfo:  %s\n", result.IndexDigest, result.InfoDigest)
		return nil
	},
}

var pluginSchemasCmd = &cobra.Command{
	Use:   "schemas <source-dir>",
	Short: "Print the asset schemas declared in a plugin's source",
	Long: `Print the same asset-schema JSON that ships as a layer on a
plugin's OCI info bundle, without pushing anything.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		schemas, err := release.ScanAssetSchemas(args[0])
		if err != nil {
			return err
		}
		enc := json.NewEncoder(cmd.OutOrStdout())
		enc.SetIndent("", "  ")
		return enc.Encode(schemas)
	},
}

func init() {
	pluginPushCmd.Flags().StringVar(&pluginPushSource, "source", ".", "Plugin source directory (README.md, *.go)")
	pluginPushCmd.Flags().StringVar(&pluginPushDist, "dist", "dist", "Directory containing pre-built per-platform binaries")
	pluginPushCmd.Flags().StringVar(&pluginPushRepoURL, "repo-url", "", "Value for the org.opencontainers.image.source annotation")
	pluginPushCmd.Flags().BoolVar(&pluginPushPlainHTTP, "plain-http", false, "Allow non-TLS registries (for local testing)")

	pluginCmd.AddCommand(pluginPushCmd)
	pluginCmd.AddCommand(pluginSchemasCmd)
	rootCmd.AddCommand(pluginCmd)
}
