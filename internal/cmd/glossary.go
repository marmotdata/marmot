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

var glossaryCmd = &cobra.Command{
	Use:   "glossary",
	Short: "Manage glossary terms",
}

var glossaryListCmd = &cobra.Command{
	Use:   "list",
	Short: "List glossary terms",
	RunE: func(cmd *cobra.Command, args []string) error {
		limit, _ := cmd.Flags().GetInt("limit")
		offset, _ := cmd.Flags().GetInt("offset")
		p := getPrinter()
		c, err := newClient()
		if err != nil {
			return err
		}

		resp, err := c.Glossary.List(cmd.Context(), marmot.GlossaryListOptions{Limit: int64(limit), Offset: int64(offset)})
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

		t := output.NewTable("ID", "NAME", "DEFINITION")
		for _, term := range resp.Terms {
			def := term.Definition
			if len(def) > 60 {
				def = def[:57] + "..."
			}
			t.AddRow(term.ID, term.Name, def)
		}
		t.SetFooter("Showing %d of %d terms", len(resp.Terms), resp.Total)
		p.PrintTable(t)
		return nil
	},
}

var glossaryGetCmd = &cobra.Command{
	Use:   "get <id>",
	Short: "Get glossary term details",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		p := getPrinter()
		c, err := newClient()
		if err != nil {
			return err
		}

		term, err := c.Glossary.Get(cmd.Context(), args[0])
		if err != nil {
			return err
		}

		if p.IsRaw() {
			data, err := marshalPayload(term)
			if err != nil {
				return err
			}
			return p.PrintRaw(data)
		}

		t := output.NewTable("FIELD", "VALUE")
		t.AddRow("ID", term.ID)
		t.AddRow("Name", term.Name)
		t.AddRow("Definition", term.Definition)
		t.AddRow("Description", term.Description)
		t.AddRow("Parent Term ID", term.ParentTermID)
		t.AddRow("Created", term.CreatedAt)
		t.AddRow("Updated", term.UpdatedAt)
		p.PrintTable(t)
		return nil
	},
}

var glossaryCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a new glossary term",
	RunE: func(cmd *cobra.Command, args []string) error {
		name, _ := cmd.Flags().GetString("name")
		definition, _ := cmd.Flags().GetString("definition")
		description, _ := cmd.Flags().GetString("description")
		parentID, _ := cmd.Flags().GetString("parent-id")

		c, err := newClient()
		if err != nil {
			return err
		}

		term, err := c.Glossary.Create(cmd.Context(), marmot.CreateTermInput{
			Name:         name,
			Definition:   definition,
			Description:  description,
			ParentTermID: parentID,
		})
		if err != nil {
			return err
		}

		fmt.Printf("Glossary term created: %s (ID: %s)\n", term.Name, term.ID)
		return nil
	},
}

var glossaryUpdateCmd = &cobra.Command{
	Use:   "update <id>",
	Short: "Update a glossary term",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := newClient()
		if err != nil {
			return err
		}

		in := marmot.UpdateTermInput{}
		if cmd.Flags().Changed("name") {
			in.Name, _ = cmd.Flags().GetString("name")
		}
		if cmd.Flags().Changed("definition") {
			in.Definition, _ = cmd.Flags().GetString("definition")
		}
		if cmd.Flags().Changed("description") {
			in.Description, _ = cmd.Flags().GetString("description")
		}
		if cmd.Flags().Changed("parent-id") {
			in.ParentTermID, _ = cmd.Flags().GetString("parent-id")
		}

		term, err := c.Glossary.Update(cmd.Context(), args[0], in)
		if err != nil {
			return err
		}

		fmt.Printf("Glossary term updated: %s (ID: %s)\n", term.Name, term.ID)
		return nil
	},
}

var glossaryDeleteCmd = &cobra.Command{
	Use:   "delete <id>",
	Short: "Delete a glossary term",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		yes, _ := cmd.Flags().GetBool("yes")
		c, err := newClient()
		if err != nil {
			return err
		}

		if !yes {
			fmt.Printf("Are you sure you want to delete glossary term %s? (y/N): ", args[0])
			reader := bufio.NewReader(os.Stdin)
			response, _ := reader.ReadString('\n')
			response = strings.TrimSpace(strings.ToLower(response))
			if response != "y" && response != "yes" {
				fmt.Println("Cancelled.")
				return nil
			}
		}

		if err := c.Glossary.Delete(cmd.Context(), args[0]); err != nil {
			return err
		}

		fmt.Printf("Glossary term %s deleted.\n", args[0])
		return nil
	},
}

var glossarySearchCmd = &cobra.Command{
	Use:   "search <query>",
	Short: "Search glossary terms",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		limit, _ := cmd.Flags().GetInt("limit")
		offset, _ := cmd.Flags().GetInt("offset")
		p := getPrinter()
		c, err := newClient()
		if err != nil {
			return err
		}

		resp, err := c.Glossary.Search(cmd.Context(), marmot.GlossarySearchOptions{
			Query:  args[0],
			Limit:  int64(limit),
			Offset: int64(offset),
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

		t := output.NewTable("ID", "NAME", "DEFINITION")
		for _, term := range resp.Terms {
			def := term.Definition
			if len(def) > 60 {
				def = def[:57] + "..."
			}
			t.AddRow(term.ID, term.Name, def)
		}
		t.SetFooter("Showing %d of %d results", len(resp.Terms), resp.Total)
		p.PrintTable(t)
		return nil
	},
}

var glossaryTagsCmd = &cobra.Command{
	Use:   "tags <term-id>",
	Short: "Manage tags on a glossary term",
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) == 0 {
			return cmd.Help()
		}
		return listGlossaryTermTags(cmd.Context(), args[0])
	},
}

func listGlossaryTermTags(ctx context.Context, termID string) error {
	p := getPrinter()
	c, err := newClient()
	if err != nil {
		return err
	}

	tags, err := c.Glossary.ListTermTags(ctx, termID)
	if err != nil {
		return err
	}

	if p.IsRaw() {
		return p.PrintJSON(tags)
	}

	if len(tags) == 0 {
		fmt.Println("No tags found on this glossary term.")
		return nil
	}

	t := output.NewTable("ID", "NAME", "DESCRIPTION")
	for _, tag := range tags {
		t.AddRow(tag.ID, tag.Name, tag.Description)
	}
	p.PrintTable(t)
	return nil
}

var glossaryTagsAddCmd = &cobra.Command{
	Use:   "add <term-id>",
	Short: "Add a tag to a glossary term",
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

		if err := c.Glossary.AddTermTag(cmd.Context(), args[0], tagID); err != nil {
			return err
		}

		fmt.Printf("Tag added to glossary term %s.\n", args[0])
		return nil
	},
}

var glossaryTagsRemoveCmd = &cobra.Command{
	Use:   "remove <term-id>",
	Short: "Remove a tag from a glossary term",
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

		if err := c.Glossary.RemoveTermTag(cmd.Context(), args[0], tagID); err != nil {
			return err
		}

		fmt.Printf("Tag removed from glossary term %s.\n", args[0])
		return nil
	},
}

var glossaryTagsSetCmd = &cobra.Command{
	Use:   "set <term-id>",
	Short: "Replace all tags on a glossary term",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		tagIDs, _ := cmd.Flags().GetStringSlice("tag-ids")

		c, err := newClient()
		if err != nil {
			return err
		}

		if err := c.Glossary.SetTermTags(cmd.Context(), args[0], tagIDs); err != nil {
			return err
		}

		fmt.Printf("Tags replaced on glossary term %s.\n", args[0])
		return nil
	},
}

func init() {
	glossaryListCmd.Flags().Int("limit", 20, "Maximum number of results")
	glossaryListCmd.Flags().Int("offset", 0, "Offset for pagination")

	glossaryCreateCmd.Flags().String("name", "", "Term name (required)")
	glossaryCreateCmd.Flags().String("definition", "", "Term definition (required)")
	glossaryCreateCmd.Flags().String("description", "", "Term description")
	glossaryCreateCmd.Flags().String("parent-id", "", "Parent term ID")
	glossaryCreateCmd.MarkFlagRequired("name")
	glossaryCreateCmd.MarkFlagRequired("definition")

	glossaryUpdateCmd.Flags().String("name", "", "Term name")
	glossaryUpdateCmd.Flags().String("definition", "", "Term definition")
	glossaryUpdateCmd.Flags().String("description", "", "Term description")
	glossaryUpdateCmd.Flags().String("parent-id", "", "Parent term ID")

	glossaryDeleteCmd.Flags().BoolP("yes", "y", false, "Skip confirmation prompt")

	glossarySearchCmd.Flags().Int("limit", 20, "Maximum number of results")
	glossarySearchCmd.Flags().Int("offset", 0, "Offset for pagination")

	// tags
	glossaryTagsAddCmd.Flags().String("tag-id", "", "Tag ID to add (required)")
	glossaryTagsAddCmd.MarkFlagRequired("tag-id")
	glossaryTagsRemoveCmd.Flags().String("tag-id", "", "Tag ID to remove (required)")
	glossaryTagsRemoveCmd.MarkFlagRequired("tag-id")
	glossaryTagsSetCmd.Flags().StringSlice("tag-ids", nil, "Tag IDs to set (replaces all existing tags)")

	glossaryTagsCmd.AddCommand(glossaryTagsAddCmd)
	glossaryTagsCmd.AddCommand(glossaryTagsRemoveCmd)
	glossaryTagsCmd.AddCommand(glossaryTagsSetCmd)

	glossaryCmd.AddCommand(glossaryListCmd)
	glossaryCmd.AddCommand(glossaryGetCmd)
	glossaryCmd.AddCommand(glossaryCreateCmd)
	glossaryCmd.AddCommand(glossaryUpdateCmd)
	glossaryCmd.AddCommand(glossaryDeleteCmd)
	glossaryCmd.AddCommand(glossarySearchCmd)
	glossaryCmd.AddCommand(glossaryTagsCmd)
	rootCmd.AddCommand(glossaryCmd)
}
