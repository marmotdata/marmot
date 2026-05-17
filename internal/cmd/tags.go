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

var tagsCmd = &cobra.Command{
	Use:   "tags",
	Short: "Manage tags in the data catalog",
}

var tagsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all tags",
	RunE: func(cmd *cobra.Command, args []string) error {
		p := getPrinter()
		c, err := newClient()
		if err != nil {
			return err
		}

		tagList, err := c.Tags.List(cmd.Context(), marmot.TagsListOptions{})
		if err != nil {
			return err
		}

		if p.IsRaw() {
			return p.PrintJSON(tagList)
		}

		if len(tagList) == 0 {
			fmt.Println("No tags found.")
			return nil
		}

		t := output.NewTable("ID", "NAME", "DESCRIPTION")
		for _, tag := range tagList {
			t.AddRow(tag.ID, tag.Name, tag.Description)
		}
		p.PrintTable(t)
		return nil
	},
}

var tagsGetCmd = &cobra.Command{
	Use:   "get <id>",
	Short: "Get tag details",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		p := getPrinter()
		c, err := newClient()
		if err != nil {
			return err
		}

		tag, err := c.Tags.Get(cmd.Context(), args[0])
		if err != nil {
			return err
		}

		if p.IsRaw() {
			return p.PrintJSON(tag)
		}

		t := output.NewTable("FIELD", "VALUE")
		t.AddRow("ID", tag.ID)
		t.AddRow("Name", tag.Name)
		if tag.Description != "" {
			t.AddRow("Description", tag.Description)
		}
		t.AddRow("Created", tag.CreatedAt)
		t.AddRow("Updated", tag.UpdatedAt)
		p.PrintTable(t)
		return nil
	},
}

var tagsCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a new tag",
	RunE: func(cmd *cobra.Command, args []string) error {
		name, _ := cmd.Flags().GetString("name")
		description, _ := cmd.Flags().GetString("description")

		if name == "" {
			return fmt.Errorf("--name is required")
		}

		c, err := newClient()
		if err != nil {
			return err
		}

		tag, err := c.Tags.Create(cmd.Context(), marmot.CreateTagInput{
			Name:        name,
			Description: description,
		})
		if err != nil {
			return err
		}

		fmt.Printf("Tag created: %s (ID: %s)\n", tag.Name, tag.ID)
		return nil
	},
}

var tagsUpdateCmd = &cobra.Command{
	Use:   "update <id>",
	Short: "Update a tag",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		in := marmot.UpdateTagInput{}

		if cmd.Flags().Changed("name") {
			in.Name, _ = cmd.Flags().GetString("name")
		}
		if cmd.Flags().Changed("description") {
			in.Description, _ = cmd.Flags().GetString("description")
		}

		if in.Name == "" && in.Description == "" {
			return fmt.Errorf("at least one of --name or --description must be provided")
		}

		c, err := newClient()
		if err != nil {
			return err
		}

		tag, err := c.Tags.Update(cmd.Context(), args[0], in)
		if err != nil {
			return err
		}

		fmt.Printf("Tag updated: %s (ID: %s)\n", tag.Name, tag.ID)
		return nil
	},
}

var tagsDeleteCmd = &cobra.Command{
	Use:   "delete <id>",
	Short: "Delete a tag",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		yes, _ := cmd.Flags().GetBool("yes")

		if !yes {
			fmt.Printf("Are you sure you want to delete tag %s? This will remove it from all associated assets. (y/N): ", args[0])
			reader := bufio.NewReader(os.Stdin)
			response, _ := reader.ReadString('\n')
			response = strings.TrimSpace(strings.ToLower(response))
			if response != "y" && response != "yes" {
				fmt.Println("Cancelled.")
				return nil
			}
		}

		c, err := newClient()
		if err != nil {
			return err
		}

		if err := c.Tags.Delete(cmd.Context(), args[0]); err != nil {
			return err
		}

		fmt.Printf("Tag %s deleted.\n", args[0])
		return nil
	},
}

func init() {
	tagsCreateCmd.Flags().String("name", "", "Tag name (required)")
	tagsCreateCmd.Flags().String("description", "", "Tag description")
	tagsCreateCmd.MarkFlagRequired("name")

	tagsUpdateCmd.Flags().String("name", "", "Tag name")
	tagsUpdateCmd.Flags().String("description", "", "Tag description")

	tagsDeleteCmd.Flags().BoolP("yes", "y", false, "Skip confirmation prompt")

	tagsCmd.AddCommand(tagsListCmd)
	tagsCmd.AddCommand(tagsGetCmd)
	tagsCmd.AddCommand(tagsCreateCmd)
	tagsCmd.AddCommand(tagsUpdateCmd)
	tagsCmd.AddCommand(tagsDeleteCmd)
	rootCmd.AddCommand(tagsCmd)
}