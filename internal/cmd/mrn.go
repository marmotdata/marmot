package cmd

import (
	"fmt"
	"strings"

	"github.com/marmotdata/marmot/internal/cmd/output"
	marmot "github.com/marmotdata/marmot/sdk/go"
	"github.com/spf13/cobra"
)

var assetsIdentityCmd = &cobra.Command{
	Use:   "identity <id>",
	Short: "Show an asset's MRN, the MRNs it used to have and which writers describe it",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		p := getPrinter()
		c, err := newClient()
		if err != nil {
			return err
		}
		id, err := c.Assets.Identity(cmd.Context(), args[0])
		if err != nil {
			return err
		}
		if p.IsRaw() {
			data, err := marshalPayload(id)
			if err != nil {
				return err
			}
			return p.PrintRaw(data)
		}

		t := output.NewTable("FIELD", "VALUE")
		t.AddRow("MRN", id.Mrn)
		if id.MrnParts != nil && id.MrnParts.Zone != "" {
			t.AddRow("Zone", id.MrnParts.Zone)
		}
		for i, alias := range id.Aliases {
			label := ""
			if i == 0 {
				label = "Previous MRNs"
			}
			t.AddRow(label, alias)
		}
		p.PrintTable(t)

		w := output.NewTable("WRITER", "ROLE", "LAST SEEN")
		for _, contribution := range id.Contributions {
			w.AddRow(contribution.Writer, contribution.Role, contribution.LastSeenAt)
		}
		p.PrintTable(w)
		return nil
	},
}

var assetsMergeCmd = &cobra.Command{
	Use:   "merge <id> --into <id>",
	Short: "Merge an asset into another that turns out to be the same thing",
	Long:  "Folds the asset into the one named by --into. Owners, glossary terms, data products, subscriptions, lineage and documentation move across, and the merged asset's MRN keeps working as an alias.",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		into, _ := cmd.Flags().GetString("into")
		if into == "" {
			return fmt.Errorf("--into is required")
		}
		if err := confirm(cmd, fmt.Sprintf("Merge asset %s into %s?", args[0], into)); err != nil {
			return err
		}
		c, err := newClient()
		if err != nil {
			return err
		}
		merged, err := c.Assets.Merge(cmd.Context(), args[0], into)
		if err != nil {
			return err
		}
		p := getPrinter()
		if p.IsRaw() {
			data, err := marshalPayload(merged)
			if err != nil {
				return err
			}
			return p.PrintRaw(data)
		}
		p.PrintMessage(fmt.Sprintf("Merged %s into %s (%s)", args[0], merged.ID, merged.Mrn))
		return nil
	},
}

var assetsUpsertCmd = &cobra.Command{
	Use:   "upsert <mrn>",
	Short: "Create the asset an MRN names, or update what you say about it if it exists",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name, _ := cmd.Flags().GetString("name")
		typ, _ := cmd.Flags().GetString("type")
		providers, _ := cmd.Flags().GetStringSlice("providers")
		description, _ := cmd.Flags().GetString("description")
		tags, _ := cmd.Flags().GetStringSlice("tags")
		c, err := newClient()
		if err != nil {
			return err
		}
		a, created, err := c.Assets.Upsert(cmd.Context(), marmot.UpsertAssetInput{
			MRN: args[0], Name: name, Type: typ, Providers: providers, Description: description, Tags: tags,
		})
		if err != nil {
			return err
		}
		p := getPrinter()
		if p.IsRaw() {
			data, err := marshalPayload(a)
			if err != nil {
				return err
			}
			return p.PrintRaw(data)
		}
		verb := "Updated"
		if created {
			verb = "Created"
		}
		p.PrintMessage(fmt.Sprintf("%s %s (%s)", verb, a.Mrn, a.ID))
		return nil
	},
}

var mrnCmd = &cobra.Command{
	Use:   "mrn",
	Short: "Work with Marmot Resource Names",
}

var mrnResolveCmd = &cobra.Command{
	Use:   "resolve <mrn>",
	Short: "Show what an MRN names, including MRNs something used to have",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := newClient()
		if err != nil {
			return err
		}
		r, err := c.Assets.ResolveMRN(cmd.Context(), args[0])
		if err != nil {
			return err
		}
		p := getPrinter()
		if p.IsRaw() {
			data, err := marshalPayload(r)
			if err != nil {
				return err
			}
			return p.PrintRaw(data)
		}
		t := output.NewTable("FIELD", "VALUE")
		t.AddRow("Kind", r.Kind)
		t.AddRow("ID", r.ID)
		t.AddRow("MRN", r.Mrn)
		if r.Moved {
			t.AddRow("Moved", "yes, the MRN you gave is one it used to have")
		}
		p.PrintTable(t)
		return nil
	},
}

var zonesCmd = &cobra.Command{
	Use:   "zones",
	Short: "Manage which zone each host belongs to",
	Long:  "Plugins report the hosts they connect to and the zone each belongs to. Bind a host yourself when a plugin can't report it, so lineage from Kafka Connect, OpenLineage and similar tools that only know an address lands on the right system's assets.",
}

var zonesListCmd = &cobra.Command{
	Use:   "list",
	Short: "List zone bindings",
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := newClient()
		if err != nil {
			return err
		}
		bindings, err := c.Assets.ZoneBindings(cmd.Context())
		if err != nil {
			return err
		}
		p := getPrinter()
		if p.IsRaw() {
			data, err := marshalPayload(bindings)
			if err != nil {
				return err
			}
			return p.PrintRaw(data)
		}
		t := output.NewTable("PROVIDER", "HOST", "ZONE", "ORIGIN")
		for _, b := range bindings {
			zone := b.Zone
			if zone == "" {
				zone = "(none)"
			}
			t.AddRow(b.Provider, b.Host, zone, b.Origin)
		}
		t.SetFooter("%d bindings", len(bindings))
		p.PrintTable(t)
		return nil
	},
}

var zonesBindCmd = &cobra.Command{
	Use:   "bind <provider> <host:port> <zone>",
	Short: "Bind a host to a zone",
	Args:  cobra.ExactArgs(3),
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := newClient()
		if err != nil {
			return err
		}
		if _, err := c.Assets.BindZone(cmd.Context(), args[0], args[1], args[2]); err != nil {
			return err
		}
		getPrinter().PrintMessage(fmt.Sprintf("Bound %s %s to zone %s", args[0], strings.ToLower(args[1]), args[2]))
		return nil
	},
}

var zonesUnbindCmd = &cobra.Command{
	Use:   "unbind <provider> <host:port>",
	Short: "Remove a host's zone binding",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := newClient()
		if err != nil {
			return err
		}
		if err := c.Assets.UnbindZone(cmd.Context(), args[0], args[1]); err != nil {
			return err
		}
		getPrinter().PrintMessage(fmt.Sprintf("Removed the binding for %s %s", args[0], strings.ToLower(args[1])))
		return nil
	},
}

func init() {
	assetsMergeCmd.Flags().String("into", "", "ID of the asset to keep")
	assetsMergeCmd.Flags().BoolP("yes", "y", false, "Skip confirmation prompt")

	assetsUpsertCmd.Flags().String("name", "", "Asset name (defaults to the MRN's name)")
	assetsUpsertCmd.Flags().String("type", "", "Asset type (defaults to the MRN's type)")
	assetsUpsertCmd.Flags().StringSlice("providers", nil, "Providers (defaults to the MRN's provider)")
	assetsUpsertCmd.Flags().String("description", "", "Description")
	assetsUpsertCmd.Flags().StringSlice("tags", nil, "Tags")

	assetsCmd.AddCommand(assetsIdentityCmd)
	assetsCmd.AddCommand(assetsMergeCmd)
	assetsCmd.AddCommand(assetsUpsertCmd)

	mrnCmd.AddCommand(mrnResolveCmd)
	rootCmd.AddCommand(mrnCmd)

	zonesCmd.AddCommand(zonesListCmd)
	zonesCmd.AddCommand(zonesBindCmd)
	zonesCmd.AddCommand(zonesUnbindCmd)
	rootCmd.AddCommand(zonesCmd)
}
