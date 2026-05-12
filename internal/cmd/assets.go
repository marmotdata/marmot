package cmd

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	marmot "github.com/marmotdata/marmot/sdk/go"
	"github.com/marmotdata/marmot/internal/cmd/output"
	"github.com/spf13/cobra"
)

var assetsCmd = &cobra.Command{
	Use:   "assets",
	Short: "Manage assets in the data catalog",
}

func searchAssets(cmd *cobra.Command, query string) error {
	limit, _ := cmd.Flags().GetInt("limit")
	offset, _ := cmd.Flags().GetInt("offset")
	types, _ := cmd.Flags().GetStringSlice("types")
	providers, _ := cmd.Flags().GetStringSlice("providers")
	tags, _ := cmd.Flags().GetStringSlice("tags")

	p := getPrinter()
	c, err := newClient()
	if err != nil {
		return err
	}

	resp, err := c.Assets.Search(cmd.Context(), marmot.AssetSearchOptions{
		Query:     query,
		Types:     types,
		Providers: providers,
		Tags:      tags,
		Limit:     int64(limit),
		Offset:    int64(offset),
	})
	if err != nil {
		return err
	}

	if p.IsRaw() {
		data, err := marshalPayload(resp)
		if err != nil {
			return err
		}
		return p.PrintRaw(data)
	}

	t := output.NewTable("ID", "NAME", "TYPE", "PROVIDERS", "TAGS")
	for _, a := range resp.Assets {
		t.AddRow(
			a.ID,
			formatAssetName(a),
			a.Type,
			strings.Join(a.Providers, ", "),
			strings.Join(a.Tags, ", "),
		)
	}
	label := "assets"
	if query != "" {
		label = "results"
	}
	t.SetFooter("Showing %d of %d "+label, len(resp.Assets), resp.Total)
	p.PrintTable(t)
	return nil
}

var assetsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List assets",
	RunE: func(cmd *cobra.Command, args []string) error {
		return searchAssets(cmd, "")
	},
}

var assetsGetCmd = &cobra.Command{
	Use:   "get <id>",
	Short: "Get asset details",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		p := getPrinter()
		c, err := newClient()
		if err != nil {
			return err
		}

		a, err := c.Assets.Get(cmd.Context(), args[0])
		if err != nil {
			return err
		}

		if p.IsRaw() {
			data, err := marshalPayload(a)
			if err != nil {
				return err
			}
			return p.PrintRaw(data)
		}

		t := output.NewTable("FIELD", "VALUE")
		t.AddRow("ID", a.ID)
		t.AddRow("Name", formatAssetName(a))
		t.AddRow("Type", a.Type)
		t.AddRow("Providers", strings.Join(a.Providers, ", "))
		t.AddRow("Description", a.Description)
		t.AddRow("Tags", strings.Join(a.Tags, ", "))
		t.AddRow("MRN", a.Mrn)
		t.AddRow("Created", a.CreatedAt)
		t.AddRow("Updated", a.UpdatedAt)
		p.PrintTable(t)
		return nil
	},
}

var assetsSearchCmd = &cobra.Command{
	Use:   "search <query>",
	Short: "Search assets",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return searchAssets(cmd, args[0])
	},
}

var assetsDeleteCmd = &cobra.Command{
	Use:   "delete <id>",
	Short: "Delete an asset",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		yes, _ := cmd.Flags().GetBool("yes")
		c, err := newClient()
		if err != nil {
			return err
		}

		if !yes {
			fmt.Printf("Are you sure you want to delete asset %s? (y/N): ", args[0])
			reader := bufio.NewReader(os.Stdin)
			response, _ := reader.ReadString('\n')
			response = strings.TrimSpace(strings.ToLower(response))
			if response != "y" && response != "yes" {
				fmt.Println("Cancelled.")
				return nil
			}
		}

		if err := c.Assets.Delete(cmd.Context(), args[0]); err != nil {
			return err
		}

		fmt.Printf("Asset %s deleted.\n", args[0])
		return nil
	},
}

var assetsSummaryCmd = &cobra.Command{
	Use:   "summary",
	Short: "Show asset summary statistics",
	RunE: func(cmd *cobra.Command, args []string) error {
		p := getPrinter()
		c, err := newClient()
		if err != nil {
			return err
		}

		s, err := c.Assets.Summary(cmd.Context())
		if err != nil {
			return err
		}

		if p.IsRaw() {
			data, err := marshalPayload(s)
			if err != nil {
				return err
			}
			return p.PrintRaw(data)
		}

		fmt.Println("Assets by Type:")
		t := output.NewTable("TYPE", "COUNT")
		for typeName, info := range s.Types {
			t.AddRow(typeName, fmt.Sprintf("%d", info.Count))
		}
		p.PrintTable(t)

		fmt.Println("\nAssets by Provider:")
		t2 := output.NewTable("PROVIDER", "COUNT")
		for provider, count := range s.Providers {
			t2.AddRow(provider, fmt.Sprintf("%d", count))
		}
		p.PrintTable(t2)

		if len(s.Tags) > 0 {
			fmt.Println("\nAssets by Tag:")
			t3 := output.NewTable("TAG", "COUNT")
			for tag, count := range s.Tags {
				t3.AddRow(tag, fmt.Sprintf("%d", count))
			}
			p.PrintTable(t3)
		}
		return nil
	},
}

var assetsTagsCmd = &cobra.Command{
	Use:   "tags",
	Short: "Manage asset tags",
}

var assetsTagsAddCmd = &cobra.Command{
	Use:   "add <asset-id> <tag>",
	Short: "Add a tag to an asset",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := newClient()
		if err != nil {
			return err
		}
		if err := c.Assets.AddTag(cmd.Context(), args[0], args[1]); err != nil {
			return err
		}
		fmt.Printf("Tag %q added to asset %s.\n", args[1], args[0])
		return nil
	},
}

var assetsTagsRemoveCmd = &cobra.Command{
	Use:   "remove <asset-id> <tag>",
	Short: "Remove a tag from an asset",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := newClient()
		if err != nil {
			return err
		}
		if err := c.Assets.RemoveTag(cmd.Context(), args[0], args[1]); err != nil {
			return err
		}
		fmt.Printf("Tag %q removed from asset %s.\n", args[1], args[0])
		return nil
	},
}

var assetsOwnersCmd = &cobra.Command{
	Use:   "owners <asset-id>",
	Short: "List owners of an asset",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		p := getPrinter()
		c, err := newClient()
		if err != nil {
			return err
		}

		resp, err := c.Owners.Search(cmd.Context(), args[0], marmot.OwnerSearchOptions{})
		if err != nil {
			return err
		}

		if p.IsRaw() {
			data, err := marshalPayload(resp)
			if err != nil {
				return err
			}
			return p.PrintRaw(data)
		}

		if len(resp.Owners) == 0 {
			fmt.Println("No owners found.")
			return nil
		}

		t := output.NewTable("ID", "NAME", "TYPE")
		for _, o := range resp.Owners {
			t.AddRow(o.ID, o.Name, o.Type)
		}
		p.PrintTable(t)
		return nil
	},
}

func init() {
	assetsListCmd.Flags().Int("limit", 20, "Maximum number of results")
	assetsListCmd.Flags().Int("offset", 0, "Offset for pagination")
	assetsListCmd.Flags().StringSlice("types", nil, "Filter by asset types")
	assetsListCmd.Flags().StringSlice("providers", nil, "Filter by providers")
	assetsListCmd.Flags().StringSlice("tags", nil, "Filter by tags")

	assetsSearchCmd.Flags().Int("limit", 20, "Maximum number of results")
	assetsSearchCmd.Flags().Int("offset", 0, "Offset for pagination")
	assetsSearchCmd.Flags().StringSlice("types", nil, "Filter by asset types")
	assetsSearchCmd.Flags().StringSlice("providers", nil, "Filter by providers")
	assetsSearchCmd.Flags().StringSlice("tags", nil, "Filter by tags")

	assetsDeleteCmd.Flags().BoolP("yes", "y", false, "Skip confirmation prompt")

	assetsTagsCmd.AddCommand(assetsTagsAddCmd)
	assetsTagsCmd.AddCommand(assetsTagsRemoveCmd)

	assetsCmd.AddCommand(assetsListCmd)
	assetsCmd.AddCommand(assetsGetCmd)
	assetsCmd.AddCommand(assetsSearchCmd)
	assetsCmd.AddCommand(assetsDeleteCmd)
	assetsCmd.AddCommand(assetsSummaryCmd)
	assetsCmd.AddCommand(assetsTagsCmd)
	assetsCmd.AddCommand(assetsOwnersCmd)
	rootCmd.AddCommand(assetsCmd)
}
