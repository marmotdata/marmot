package cmd

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"

	"github.com/marmotdata/marmot"
	"github.com/spf13/cobra"
)

var skillsCmd = &cobra.Command{
	Use:   "skills",
	Short: "Install the Marmot skill for AI agents",
}

var skillsInstallCmd = &cobra.Command{
	Use:   "install",
	Short: "Install the skill that teaches agents this CLI, the API and the MCP server",
	Long: `Writes the Marmot skill bundled with this binary, so it always matches the CLI you have.
By default it goes to ~/.claude/skills/marmot for every project; --project installs it into
the current project instead, and --dir anywhere else. Run it again after upgrading.`,
	Example: `  marmot skills install
  marmot skills install --project
  marmot skills install --dir ~/.agents/skills/marmot
  marmot skills install --print > SKILL.md`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		if printOnly, _ := cmd.Flags().GetBool("print"); printOnly {
			_, err := os.Stdout.Write(marmot.Skill)
			return err
		}
		dir, err := skillDir(cmd)
		if err != nil {
			return err
		}
		path := filepath.Join(dir, "SKILL.md")
		existing, err := os.ReadFile(path)
		if err == nil && bytes.Equal(existing, marmot.Skill) {
			fmt.Fprintf(os.Stderr, "%s is already up to date.\n", path)
			return nil
		}
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("creating %s: %w", dir, err)
		}
		if err := os.WriteFile(path, marmot.Skill, 0o644); err != nil { //nolint:gosec // G306: a skill file agents read, not a secret
			return fmt.Errorf("writing %s: %w", path, err)
		}
		verb := "Installed"
		if existing != nil {
			verb = "Updated"
		}
		fmt.Fprintf(os.Stderr, "%s the Marmot skill at %s\n", verb, path)
		return nil
	},
}

func skillDir(cmd *cobra.Command) (string, error) {
	if dir, _ := cmd.Flags().GetString("dir"); dir != "" {
		return dir, nil
	}
	if project, _ := cmd.Flags().GetBool("project"); project {
		return filepath.Join(".claude", "skills", "marmot"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("finding your home directory: %w", err)
	}
	return filepath.Join(home, ".claude", "skills", "marmot"), nil
}

func init() {
	skillsInstallCmd.Flags().Bool("project", false, "Install into ./.claude/skills/marmot for this project only")
	skillsInstallCmd.Flags().String("dir", "", "Install into this directory instead")
	skillsInstallCmd.Flags().Bool("print", false, "Print the skill to stdout instead of installing it")
	skillsInstallCmd.MarkFlagsMutuallyExclusive("project", "dir", "print")
	skillsCmd.AddCommand(skillsInstallCmd)
	rootCmd.AddCommand(skillsCmd)
}
