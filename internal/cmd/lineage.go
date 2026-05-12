package cmd

import (
	"fmt"

	marmot "github.com/marmotdata/marmot/sdk/go"
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
		c, err := newClient()
		if err != nil {
			return err
		}

		graph, err := c.Lineage.Get(cmd.Context(), args[0], marmot.LineageOptions{Limit: int64(depth)})
		if err != nil {
			return err
		}

		if p.IsRaw() {
			data, err := marshalPayload(graph)
			if err != nil {
				return err
			}
			return p.PrintRaw(data)
		}

		fmt.Printf("Lineage for asset %s\n\n", args[0])

		if len(graph.Nodes) > 0 {
			fmt.Println("Nodes:")
			t := output.NewTable("ID", "TYPE", "NAME", "DEPTH")
			for _, n := range graph.Nodes {
				name := n.ID
				if n.Asset != nil {
					name = formatAssetName(n.Asset)
				}
				t.AddRow(n.ID, n.Type, name, fmt.Sprintf("%d", n.Depth))
			}
			p.PrintTable(t)
		}

		if len(graph.Edges) > 0 {
			fmt.Println("\nEdges:")
			t := output.NewTable("SOURCE", "TARGET", "TYPE")
			for _, e := range graph.Edges {
				t.AddRow(e.Source, e.Target, e.Type)
			}
			p.PrintTable(t)
		}

		if len(graph.Nodes) == 0 && len(graph.Edges) == 0 {
			fmt.Println("No lineage found.")
		}

		fmt.Printf("\n%d nodes, %d edges\n", len(graph.Nodes), len(graph.Edges))
		return nil
	},
}

func init() {
	lineageGetCmd.Flags().Int("depth", 0, "Maximum traversal depth (0 = default)")

	lineageCmd.AddCommand(lineageGetCmd)
	rootCmd.AddCommand(lineageCmd)
}
