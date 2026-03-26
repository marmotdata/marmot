package cmd

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/marmotdata/marmot/client/client/assets"
	"github.com/marmotdata/marmot/client/client/owners"
	"github.com/marmotdata/marmot/client/models"
	"github.com/marmotdata/marmot/internal/cmd/output"
	"github.com/spf13/cobra"
)

var assetsCmd = &cobra.Command{
	Use:   "assets",
	Short: "Manage assets in the data catalog",
}

var assetsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List assets",
	RunE: func(cmd *cobra.Command, args []string) error {
		limit, _ := cmd.Flags().GetInt("limit")
		offset, _ := cmd.Flags().GetInt("offset")
		types, _ := cmd.Flags().GetStringSlice("types")
		providers, _ := cmd.Flags().GetStringSlice("providers")
		tags, _ := cmd.Flags().GetStringSlice("tags")

		p := getPrinter()
		c := newSwaggerClient()

		params := assets.NewGetAssetsSearchParams()
		params.SetLimit(int64Ptr(limit))
		params.SetOffset(int64Ptr(offset))
		if len(types) > 0 {
			params.SetTypes(types)
		}
		if len(providers) > 0 {
			params.SetServices(providers)
		}
		if len(tags) > 0 {
			params.SetTags(tags)
		}

		resp, err := c.Assets.GetAssetsSearch(params)
		if err != nil {
			return err
		}

		if p.IsRaw() {
			data, err := marshalPayload(resp.Payload)
			if err != nil {
				return err
			}
			return p.PrintRaw(data)
		}

		t := output.NewTable("ID", "NAME", "TYPE", "PROVIDERS", "TAGS")
		for _, a := range resp.Payload.Assets {
			t.AddRow(
				a.ID,
				formatAssetName(a),
				a.Type,
				strings.Join(a.Providers, ", "),
				strings.Join(a.Tags, ", "),
			)
		}
		t.SetFooter("Showing %d of %d assets", len(resp.Payload.Assets), resp.Payload.Total)
		p.PrintTable(t)
		return nil
	},
}

var assetsGetCmd = &cobra.Command{
	Use:   "get <id>",
	Short: "Get asset details",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		p := getPrinter()
		c := newSwaggerClient()

		params := assets.NewGetAssetsIDParams()
		params.SetID(args[0])

		resp, err := c.Assets.GetAssetsID(params)
		if err != nil {
			return err
		}

		if p.IsRaw() {
			data, err := marshalPayload(resp.Payload)
			if err != nil {
				return err
			}
			return p.PrintRaw(data)
		}

		a := resp.Payload
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
		limit, _ := cmd.Flags().GetInt("limit")
		offset, _ := cmd.Flags().GetInt("offset")
		types, _ := cmd.Flags().GetStringSlice("types")
		providers, _ := cmd.Flags().GetStringSlice("providers")
		tags, _ := cmd.Flags().GetStringSlice("tags")

		p := getPrinter()
		c := newSwaggerClient()

		params := assets.NewGetAssetsSearchParams()
		params.SetQ(strPtr(args[0]))
		params.SetLimit(int64Ptr(limit))
		params.SetOffset(int64Ptr(offset))
		if len(types) > 0 {
			params.SetTypes(types)
		}
		if len(providers) > 0 {
			params.SetServices(providers)
		}
		if len(tags) > 0 {
			params.SetTags(tags)
		}

		resp, err := c.Assets.GetAssetsSearch(params)
		if err != nil {
			return err
		}

		if p.IsRaw() {
			data, err := marshalPayload(resp.Payload)
			if err != nil {
				return err
			}
			return p.PrintRaw(data)
		}

		t := output.NewTable("ID", "NAME", "TYPE", "PROVIDERS", "TAGS")
		for _, a := range resp.Payload.Assets {
			t.AddRow(
				a.ID,
				formatAssetName(a),
				a.Type,
				strings.Join(a.Providers, ", "),
				strings.Join(a.Tags, ", "),
			)
		}
		t.SetFooter("Showing %d of %d results", len(resp.Payload.Assets), resp.Payload.Total)
		p.PrintTable(t)
		return nil
	},
}

var assetsDeleteCmd = &cobra.Command{
	Use:   "delete <id>",
	Short: "Delete an asset",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		yes, _ := cmd.Flags().GetBool("yes")
		c := newSwaggerClient()

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

		params := assets.NewDeleteAssetsIDParams()
		params.SetID(args[0])
		if _, err := c.Assets.DeleteAssetsID(params); err != nil {
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
		c := newSwaggerClient()

		resp, err := c.Assets.GetAssetsSummary(assets.NewGetAssetsSummaryParams())
		if err != nil {
			return err
		}

		if p.IsRaw() {
			data, err := marshalPayload(resp.Payload)
			if err != nil {
				return err
			}
			return p.PrintRaw(data)
		}

		s := resp.Payload

		fmt.Println("Assets by Type:")
		t := output.NewTable("TYPE", "COUNT")
		for typeName, count := range s.Types {
			t.AddRow(typeName, fmt.Sprintf("%d", count))
		}
		p.PrintTable(t)

		fmt.Println("\nAssets by Provider:")
		t2 := output.NewTable("PROVIDER", "COUNT")
		for provider, count := range s.Services {
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
		c := newSwaggerClient()
		params := assets.NewPostAssetsIDTagsParams()
		params.SetID(args[0])
		params.SetTag(&models.V1AssetsTagRequest{Tag: strPtr(args[1])})
		if _, err := c.Assets.PostAssetsIDTags(params); err != nil {
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
		c := newSwaggerClient()
		params := assets.NewDeleteAssetsIDTagsParams()
		params.SetID(args[0])
		params.SetTag(&models.V1AssetsTagRequest{Tag: strPtr(args[1])})
		if _, err := c.Assets.DeleteAssetsIDTags(params); err != nil {
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
		c := newSwaggerClient()

		params := owners.NewGetOwnersSearchParams()
		params.SetQ(args[0])

		resp, err := c.Owners.GetOwnersSearch(params)
		if err != nil {
			return err
		}

		if p.IsRaw() {
			data, err := marshalPayload(resp.Payload)
			if err != nil {
				return err
			}
			return p.PrintRaw(data)
		}

		if len(resp.Payload.Owners) == 0 {
			fmt.Println("No owners found.")
			return nil
		}

		t := output.NewTable("ID", "NAME", "TYPE")
		for _, o := range resp.Payload.Owners {
			t.AddRow(o.ID, o.Name, o.Type)
		}
		p.PrintTable(t)
		return nil
	},
}

func init() {
	// list
	assetsListCmd.Flags().Int("limit", 20, "Maximum number of results")
	assetsListCmd.Flags().Int("offset", 0, "Offset for pagination")
	assetsListCmd.Flags().StringSlice("types", nil, "Filter by asset types")
	assetsListCmd.Flags().StringSlice("providers", nil, "Filter by providers")
	assetsListCmd.Flags().StringSlice("tags", nil, "Filter by tags")

	// search
	assetsSearchCmd.Flags().Int("limit", 20, "Maximum number of results")
	assetsSearchCmd.Flags().Int("offset", 0, "Offset for pagination")
	assetsSearchCmd.Flags().StringSlice("types", nil, "Filter by asset types")
	assetsSearchCmd.Flags().StringSlice("providers", nil, "Filter by providers")
	assetsSearchCmd.Flags().StringSlice("tags", nil, "Filter by tags")

	// delete
	assetsDeleteCmd.Flags().BoolP("yes", "y", false, "Skip confirmation prompt")

	// tags
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
