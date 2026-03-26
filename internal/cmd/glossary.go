package cmd

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/marmotdata/marmot/client/client/glossary"
	"github.com/marmotdata/marmot/client/models"
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
		c := newSwaggerClient()

		params := glossary.NewGetGlossaryListParams()
		params.SetLimit(int64Ptr(limit))
		params.SetOffset(int64Ptr(offset))

		resp, err := c.Glossary.GetGlossaryList(params)
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

		t := output.NewTable("ID", "NAME", "DEFINITION")
		for _, term := range resp.Payload.Terms {
			def := term.Definition
			if len(def) > 60 {
				def = def[:57] + "..."
			}
			t.AddRow(term.ID, term.Name, def)
		}
		t.SetFooter("Showing %d of %d terms", len(resp.Payload.Terms), resp.Payload.Total)
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
		c := newSwaggerClient()

		params := glossary.NewGetGlossaryIDParams()
		params.SetID(args[0])

		resp, err := c.Glossary.GetGlossaryID(params)
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

		term := resp.Payload
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

		c := newSwaggerClient()

		req := &models.V1GlossaryCreateTermRequest{
			Name:         strPtr(name),
			Definition:   strPtr(definition),
			Description:  description,
			ParentTermID: parentID,
		}

		params := glossary.NewPostGlossaryParams()
		params.SetTerm(req)

		resp, err := c.Glossary.PostGlossary(params)
		if err != nil {
			return err
		}

		fmt.Printf("Glossary term created: %s (ID: %s)\n", resp.Payload.Name, resp.Payload.ID)
		return nil
	},
}

var glossaryUpdateCmd = &cobra.Command{
	Use:   "update <id>",
	Short: "Update a glossary term",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		c := newSwaggerClient()

		req := &models.V1GlossaryUpdateTermRequest{}
		if cmd.Flags().Changed("name") {
			v, _ := cmd.Flags().GetString("name")
			req.Name = v
		}
		if cmd.Flags().Changed("definition") {
			v, _ := cmd.Flags().GetString("definition")
			req.Definition = v
		}
		if cmd.Flags().Changed("description") {
			v, _ := cmd.Flags().GetString("description")
			req.Description = v
		}
		if cmd.Flags().Changed("parent-id") {
			v, _ := cmd.Flags().GetString("parent-id")
			req.ParentTermID = v
		}

		params := glossary.NewPutGlossaryIDParams()
		params.SetID(args[0])
		params.SetTerm(req)

		resp, err := c.Glossary.PutGlossaryID(params)
		if err != nil {
			return err
		}

		fmt.Printf("Glossary term updated: %s (ID: %s)\n", resp.Payload.Name, resp.Payload.ID)
		return nil
	},
}

var glossaryDeleteCmd = &cobra.Command{
	Use:   "delete <id>",
	Short: "Delete a glossary term",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		yes, _ := cmd.Flags().GetBool("yes")
		c := newSwaggerClient()

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

		params := glossary.NewDeleteGlossaryIDParams()
		params.SetID(args[0])
		if _, err := c.Glossary.DeleteGlossaryID(params); err != nil {
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
		c := newSwaggerClient()

		params := glossary.NewGetGlossarySearchParams()
		params.SetQ(strPtr(args[0]))
		params.SetLimit(int64Ptr(limit))
		params.SetOffset(int64Ptr(offset))

		resp, err := c.Glossary.GetGlossarySearch(params)
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

		t := output.NewTable("ID", "NAME", "DEFINITION")
		for _, term := range resp.Payload.Terms {
			def := term.Definition
			if len(def) > 60 {
				def = def[:57] + "..."
			}
			t.AddRow(term.ID, term.Name, def)
		}
		t.SetFooter("Showing %d of %d results", len(resp.Payload.Terms), resp.Payload.Total)
		p.PrintTable(t)
		return nil
	},
}

func init() {
	// list
	glossaryListCmd.Flags().Int("limit", 20, "Maximum number of results")
	glossaryListCmd.Flags().Int("offset", 0, "Offset for pagination")

	// create
	glossaryCreateCmd.Flags().String("name", "", "Term name (required)")
	glossaryCreateCmd.Flags().String("definition", "", "Term definition (required)")
	glossaryCreateCmd.Flags().String("description", "", "Term description")
	glossaryCreateCmd.Flags().String("parent-id", "", "Parent term ID")
	glossaryCreateCmd.MarkFlagRequired("name")
	glossaryCreateCmd.MarkFlagRequired("definition")

	// update
	glossaryUpdateCmd.Flags().String("name", "", "Term name")
	glossaryUpdateCmd.Flags().String("definition", "", "Term definition")
	glossaryUpdateCmd.Flags().String("description", "", "Term description")
	glossaryUpdateCmd.Flags().String("parent-id", "", "Parent term ID")

	// delete
	glossaryDeleteCmd.Flags().BoolP("yes", "y", false, "Skip confirmation prompt")

	// search
	glossarySearchCmd.Flags().Int("limit", 20, "Maximum number of results")
	glossarySearchCmd.Flags().Int("offset", 0, "Offset for pagination")

	glossaryCmd.AddCommand(glossaryListCmd)
	glossaryCmd.AddCommand(glossaryGetCmd)
	glossaryCmd.AddCommand(glossaryCreateCmd)
	glossaryCmd.AddCommand(glossaryUpdateCmd)
	glossaryCmd.AddCommand(glossaryDeleteCmd)
	glossaryCmd.AddCommand(glossarySearchCmd)
	rootCmd.AddCommand(glossaryCmd)
}
