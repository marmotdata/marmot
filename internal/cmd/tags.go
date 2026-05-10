package cmd

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/marmotdata/marmot/client/client/tags"
	"github.com/marmotdata/marmot/client/models"
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
		c := newSwaggerClient()

		resp, err := c.Tags.GetTags(tags.NewGetTagsParams())
		if err != nil {
			return err
		}

		if p.IsRaw() {
			return p.PrintJSON(resp.Payload)
		}

		if len(resp.Payload) == 0 {
			fmt.Println("No tags found.")
			return nil
		}

		t := output.NewTable("ID", "NAME", "DESCRIPTION")
		for _, tag := range resp.Payload {
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
		c := newSwaggerClient()

		params := tags.NewGetTagsIDParams()
		params.SetID(args[0])

		resp, err := c.Tags.GetTagsID(params)
		if err != nil {
			return err
		}

		if p.IsRaw() {
			return p.PrintJSON(resp.Payload)
		}

		tag := resp.Payload
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

		c := newSwaggerClient()
		params := tags.NewPostTagsParams()
		params.SetBody(&models.V1TagsTagRequest{
			Name:        name,
			Description: description,
		})

		resp, err := c.Tags.PostTags(params)
		if err != nil {
			return err
		}

		fmt.Printf("Tag created: %s (ID: %s)\n", resp.Payload.Name, resp.Payload.ID)
		return nil
	},
}

var tagsUpdateCmd = &cobra.Command{
	Use:   "update <id>",
	Short: "Update a tag",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		req := &models.V1TagsTagRequest{}

		if cmd.Flags().Changed("name") {
			name, _ := cmd.Flags().GetString("name")
			req.Name = name
		}
		if cmd.Flags().Changed("description") {
			description, _ := cmd.Flags().GetString("description")
			req.Description = description
		}

		if req.Name == "" && req.Description == "" {
			return fmt.Errorf("at least one of --name or --description must be provided")
		}

		c := newSwaggerClient()
		params := tags.NewPutTagsIDParams()
		params.SetID(args[0])
		params.SetBody(req)

		resp, err := c.Tags.PutTagsID(params)
		if err != nil {
			return err
		}

		fmt.Printf("Tag updated: %s (ID: %s)\n", resp.Payload.Name, resp.Payload.ID)
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

		c := newSwaggerClient()
		params := tags.NewDeleteTagsIDParams()
		params.SetID(args[0])

		if _, err := c.Tags.DeleteTagsID(params); err != nil {
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
