package cmd

import (
	"fmt"

	"github.com/go-openapi/strfmt"
	"github.com/marmotdata/marmot/client/client/lineage"
	"github.com/marmotdata/marmot/internal/cmd/output"
	"github.com/spf13/cobra"
)

var lineageCmd = &cobra.Command{
	Use:   "lineage",
	Short: "View asset lineage",
}

var lineageGetCmd = &cobra.Command{
	Use:   "get <asset-id>",
	Short: "Get lineage graph for an asset",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		depth, _ := cmd.Flags().GetInt("depth")
		p := getPrinter()
		c := newSwaggerClient()

		params := lineage.NewGetLineageAssetsIDParams()
		params.SetID(strfmt.UUID(args[0]))
		if depth > 0 {
			params.SetLimit(int64Ptr(depth))
		}

		resp, err := c.Lineage.GetLineageAssetsID(params)
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

		fmt.Printf("Lineage for asset %s\n\n", args[0])

		if len(resp.Payload.Nodes) > 0 {
			fmt.Println("Nodes:")
			t := output.NewTable("ID", "TYPE", "NAME", "DEPTH")
			for _, n := range resp.Payload.Nodes {
				name := n.ID
				if n.Asset != nil {
					name = formatAssetName(n.Asset)
				}
				t.AddRow(n.ID, n.Type, name, fmt.Sprintf("%d", n.Depth))
			}
			p.PrintTable(t)
		}

		if len(resp.Payload.Edges) > 0 {
			fmt.Println("\nEdges:")
			t := output.NewTable("SOURCE", "TARGET", "TYPE")
			for _, e := range resp.Payload.Edges {
				t.AddRow(e.Source, e.Target, e.Type)
			}
			p.PrintTable(t)
		}

		if len(resp.Payload.Nodes) == 0 && len(resp.Payload.Edges) == 0 {
			fmt.Println("No lineage found.")
		}

		fmt.Printf("\n%d nodes, %d edges\n", len(resp.Payload.Nodes), len(resp.Payload.Edges))
		return nil
	},
}

func init() {
	lineageGetCmd.Flags().Int("depth", 0, "Maximum traversal depth (0 = default)")

	lineageCmd.AddCommand(lineageGetCmd)
	rootCmd.AddCommand(lineageCmd)
}
