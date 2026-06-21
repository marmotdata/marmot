package cmd

import (
	"bufio"
	"context"
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

	t := output.NewTable("ID", "NAME", "TYPE", "PROVIDERS")
	for _, a := range resp.Assets {
		t.AddRow(
			a.ID,
			formatAssetName(a),
			a.Type,
			strings.Join(a.Providers, ", "),
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

var assetsTagsCmd = &cobra.Command{
	Use:   "tags <asset-id>",
	Short: "Manage tags on an asset",
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) == 0 {
			return cmd.Help()
		}
		return listAssetTags(cmd.Context(), args[0])
	},
}

func listAssetTags(ctx context.Context, assetID string) error {
	p := getPrinter()
	c, err := newClient()
	if err != nil {
		return err
	}

	tags, err := c.Assets.ListTags(ctx, assetID)
	if err != nil {
		return err
	}

	if p.IsRaw() {
		return p.PrintJSON(tags)
	}

	if len(tags) == 0 {
		fmt.Println("No tags found on this asset.")
		return nil
	}

	t := output.NewTable("ID", "NAME", "DESCRIPTION")
	for _, tag := range tags {
		t.AddRow(tag.ID, tag.Name, tag.Description)
	}
	p.PrintTable(t)
	return nil
}

var assetsTagsAddCmd = &cobra.Command{
	Use:   "add <asset-id>",
	Short: "Add a tag to an asset",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		tagID, _ := cmd.Flags().GetString("tag-id")
		if tagID == "" {
			return fmt.Errorf("--tag-id is required")
		}

		c, err := newClient()
		if err != nil {
			return err
		}

		if err := c.Assets.AddTag(cmd.Context(), args[0], tagID); err != nil {
			return err
		}

		fmt.Printf("Tag added to asset %s.\n", args[0])
		return nil
	},
}

var assetsTagsRemoveCmd = &cobra.Command{
	Use:   "remove <asset-id>",
	Short: "Remove a tag from an asset",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		tagID, _ := cmd.Flags().GetString("tag-id")
		if tagID == "" {
			return fmt.Errorf("--tag-id is required")
		}

		c, err := newClient()
		if err != nil {
			return err
		}

		if err := c.Assets.RemoveTag(cmd.Context(), args[0], tagID); err != nil {
			return err
		}

		fmt.Printf("Tag removed from asset %s.\n", args[0])
		return nil
	},
}

var assetsTagsSetCmd = &cobra.Command{
	Use:   "set <asset-id>",
	Short: "Replace all tags on an asset",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		tagIDs, _ := cmd.Flags().GetStringSlice("tag-ids")

		c, err := newClient()
		if err != nil {
			return err
		}

		if err := c.Assets.SetTags(cmd.Context(), args[0], tagIDs); err != nil {
			return err
		}

		fmt.Printf("Tags replaced on asset %s.\n", args[0])
		return nil
	},
}

var assetsColumnTagsCmd = &cobra.Command{
	Use:   "column-tags <asset-id>",
	Short: "Manage column tags on an asset",
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) == 0 {
			return cmd.Help()
		}
		return nil
	},
}

var assetsColumnTagsSetCmd = &cobra.Command{
	Use:   "set <asset-id>",
	Short: "Replace all tags on a specific column of an asset",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		columnPath, _ := cmd.Flags().GetString("column")
		tagIDs, _ := cmd.Flags().GetStringSlice("tag-ids")

		c, err := newClient()
		if err != nil {
			return err
		}

		if err := c.Assets.SetColumnTags(cmd.Context(), args[0], columnPath, tagIDs); err != nil {
			return err
		}

		fmt.Printf("Column tags replaced on asset %s.\n", args[0])
		return nil
	},
}

var assetsColumnTagsRemoveCmd = &cobra.Command{
	Use:   "remove <asset-id>",
	Short: "Remove a tag from a specific column of an asset",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		columnPath, _ := cmd.Flags().GetString("column")
		tagID, _ := cmd.Flags().GetString("tag-id")

		c, err := newClient()
		if err != nil {
			return err
		}

		if err := c.Assets.RemoveColumnTag(cmd.Context(), args[0], columnPath, tagID); err != nil {
			return err
		}

		fmt.Printf("Column tag removed from asset %s.\n", args[0])
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

	// tags
	assetsTagsAddCmd.Flags().String("tag-id", "", "Tag ID to add (required)")
	assetsTagsAddCmd.MarkFlagRequired("tag-id")
	assetsTagsRemoveCmd.Flags().String("tag-id", "", "Tag ID to remove (required)")
	assetsTagsRemoveCmd.MarkFlagRequired("tag-id")
	assetsTagsSetCmd.Flags().StringSlice("tag-ids", nil, "Tag IDs to set (replaces all existing tags)")

	assetsTagsCmd.AddCommand(assetsTagsAddCmd)
	assetsTagsCmd.AddCommand(assetsTagsRemoveCmd)
	assetsTagsCmd.AddCommand(assetsTagsSetCmd)

	// column tags
	assetsColumnTagsSetCmd.Flags().String("column", "", "Column path (required)")
	assetsColumnTagsSetCmd.MarkFlagRequired("column")
	assetsColumnTagsSetCmd.Flags().StringSlice("tag-ids", nil, "Tag IDs to set (replaces all existing tags)")
	assetsColumnTagsSetCmd.MarkFlagRequired("tag-ids")
	assetsColumnTagsRemoveCmd.Flags().String("column", "", "Column path (required)")
	assetsColumnTagsRemoveCmd.MarkFlagRequired("column")
	assetsColumnTagsRemoveCmd.Flags().String("tag-id", "", "Tag ID to remove (required)")
	assetsColumnTagsRemoveCmd.MarkFlagRequired("tag-id")

	assetsColumnTagsCmd.AddCommand(assetsColumnTagsSetCmd)
	assetsColumnTagsCmd.AddCommand(assetsColumnTagsRemoveCmd)

	assetsCmd.AddCommand(assetsListCmd)
	assetsCmd.AddCommand(assetsGetCmd)
	assetsCmd.AddCommand(assetsSearchCmd)
	assetsCmd.AddCommand(assetsDeleteCmd)
	assetsCmd.AddCommand(assetsSummaryCmd)
	assetsCmd.AddCommand(assetsOwnersCmd)
	assetsCmd.AddCommand(assetsTagsCmd)
	assetsCmd.AddCommand(assetsColumnTagsCmd)
	rootCmd.AddCommand(assetsCmd)
}