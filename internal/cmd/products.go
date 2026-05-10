package cmd

import (
	"fmt"
	"strings"

	"github.com/marmotdata/marmot/client/client/products"
	"github.com/marmotdata/marmot/client/models"
	"github.com/marmotdata/marmot/internal/cmd/output"
	"github.com/spf13/cobra"
)

var productsCmd = &cobra.Command{
	Use:   "products",
	Short: "Manage data products",
}

var productsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List data products",
	RunE: func(cmd *cobra.Command, args []string) error {
		limit, _ := cmd.Flags().GetInt("limit")
		offset, _ := cmd.Flags().GetInt("offset")
		p := getPrinter()
		c := newSwaggerClient()

		params := products.NewGetProductsListParams()
		params.SetLimit(int64Ptr(limit))
		params.SetOffset(int64Ptr(offset))

		resp, err := c.Products.GetProductsList(params)
		if err != nil {
			return err
		}

		if p.IsRaw() {
			return p.PrintJSON(resp.Payload)
		}

		result := resp.Payload
		if len(result.DataProducts) == 0 {
			fmt.Println("No data products found.")
			return nil
		}

		t := output.NewTable("ID", "NAME", "DESCRIPTION", "ASSETS")
		for _, dp := range result.DataProducts {
			desc := dp.Description
			if len(desc) > 60 {
				desc = desc[:57] + "..."
			}
			assetCount := int(dp.AssetCount + dp.ManualAssetCount + dp.RuleAssetCount)
			t.AddRow(dp.ID, dp.Name, desc, fmt.Sprintf("%d", assetCount))
		}
		t.SetFooter("Showing %d of %d products", len(result.DataProducts), result.Total)
		p.PrintTable(t)
		return nil
	},
}

var productsGetCmd = &cobra.Command{
	Use:   "get <id>",
	Short: "Get data product details",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		p := getPrinter()
		c := newSwaggerClient()

		params := products.NewGetProductsIDParams()
		params.SetID(args[0])

		resp, err := c.Products.GetProductsID(params)
		if err != nil {
			return err
		}

		if p.IsRaw() {
			return p.PrintJSON(resp.Payload)
		}

		dp := resp.Payload
		t := output.NewTable("FIELD", "VALUE")
		t.AddRow("ID", dp.ID)
		t.AddRow("Name", dp.Name)
		if dp.Description != "" {
			t.AddRow("Description", dp.Description)
		}
		assetCount := int(dp.AssetCount + dp.ManualAssetCount + dp.RuleAssetCount)
		t.AddRow("Assets", fmt.Sprintf("%d", assetCount))
		if dp.CreatedBy != "" {
			t.AddRow("Created By", dp.CreatedBy)
		}
		t.AddRow("Created", dp.CreatedAt)
		t.AddRow("Updated", dp.UpdatedAt)
		if len(dp.Tags) > 0 {
			tagNames := make([]string, len(dp.Tags))
			for i, tag := range dp.Tags {
				tagNames[i] = tag.Name
			}
			t.AddRow("Tags", strings.Join(tagNames, ", "))
		}
		p.PrintTable(t)
		return nil
	},
}

var productsTagsCmd = &cobra.Command{
	Use:   "tags <product-id>",
	Short: "Manage tags on a data product",
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) == 0 {
			return cmd.Help()
		}
		return listProductTags(args[0])
	},
}

func listProductTags(productID string) error {
	p := getPrinter()
	c := newSwaggerClient()

	params := products.NewGetProductsTagsIDParams()
	params.SetID(productID)

	resp, err := c.Products.GetProductsTagsID(params)
	if err != nil {
		return err
	}

	if p.IsRaw() {
		return p.PrintJSON(resp.Payload)
	}

	if len(resp.Payload) == 0 {
		fmt.Println("No tags found on this data product.")
		return nil
	}

	t := output.NewTable("ID", "NAME", "DESCRIPTION")
	for _, tag := range resp.Payload {
		t.AddRow(tag.ID, tag.Name, tag.Description)
	}
	p.PrintTable(t)
	return nil
}

var productsTagsAddCmd = &cobra.Command{
	Use:   "add <product-id>",
	Short: "Add a tag to a data product",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		tagID, _ := cmd.Flags().GetString("tag-id")
		if tagID == "" {
			return fmt.Errorf("--tag-id is required")
		}

		c := newSwaggerClient()
		params := products.NewPostProductsTagsIDParams()
		params.SetID(args[0])
		params.SetBody(&models.V1DataproductsAddProductTagRequest{
			TagID: tagID,
		})

		if _, err := c.Products.PostProductsTagsID(params); err != nil {
			return err
		}

		fmt.Printf("Tag added to data product %s.\n", args[0])
		return nil
	},
}

var productsTagsRemoveCmd = &cobra.Command{
	Use:   "remove <product-id>",
	Short: "Remove a tag from a data product",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		tagID, _ := cmd.Flags().GetString("tag-id")
		if tagID == "" {
			return fmt.Errorf("--tag-id is required")
		}

		c := newSwaggerClient()
		params := products.NewDeleteProductsTagsIDParams()
		params.SetID(args[0])
		params.SetBody(&models.V1DataproductsRemoveProductTagRequest{
			TagID: tagID,
		})

		if _, err := c.Products.DeleteProductsTagsID(params); err != nil {
			return err
		}

		fmt.Printf("Tag removed from data product %s.\n", args[0])
		return nil
	},
}

var productsTagsSetCmd = &cobra.Command{
	Use:   "set <product-id>",
	Short: "Replace all tags on a data product",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		tagIDs, _ := cmd.Flags().GetStringSlice("tag-ids")

		c := newSwaggerClient()
		params := products.NewPutProductsTagsIDParams()
		params.SetID(args[0])
		params.SetBody(&models.V1DataproductsReplaceProductTagsRequest{
			TagIds: tagIDs,
		})

		if _, err := c.Products.PutProductsTagsID(params); err != nil {
			return err
		}

		fmt.Printf("Tags replaced on data product %s.\n", args[0])
		return nil
	},
}

func init() {
	productsListCmd.Flags().Int("limit", 20, "Maximum number of results")
	productsListCmd.Flags().Int("offset", 0, "Offset for pagination")

	productsTagsAddCmd.Flags().String("tag-id", "", "Tag ID to add (required)")
	productsTagsAddCmd.MarkFlagRequired("tag-id")
	productsTagsRemoveCmd.Flags().String("tag-id", "", "Tag ID to remove (required)")
	productsTagsRemoveCmd.MarkFlagRequired("tag-id")
	productsTagsSetCmd.Flags().StringSlice("tag-ids", nil, "Tag IDs to set (replaces all existing tags)")

	productsTagsCmd.AddCommand(productsTagsAddCmd)
	productsTagsCmd.AddCommand(productsTagsRemoveCmd)
	productsTagsCmd.AddCommand(productsTagsSetCmd)

	productsCmd.AddCommand(productsListCmd)
	productsCmd.AddCommand(productsGetCmd)
	productsCmd.AddCommand(productsTagsCmd)
	rootCmd.AddCommand(productsCmd)
}
