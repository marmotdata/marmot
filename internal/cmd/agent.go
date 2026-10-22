package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

// Access says what a command can change, so read-only mode can block it and `marmot agent schema` can tell an agent before it runs anything.
type Access string

const (
	// AccessRead only reads from the server or prints local information.
	AccessRead Access = "read"
	// AccessWrite changes state on the server or runs a long-lived process against it.
	AccessWrite Access = "write"
	// AccessLocal changes local configuration or credentials only.
	AccessLocal Access = "local"
)

// commandAccess classifies every runnable command by its path without the leading "marmot". TestEveryCommandHasAccess fails when a command is missing, so a new command cannot slip past read-only mode unclassified.
var commandAccess = map[string]Access{
	"query history":                   AccessRead,
	"query run":                       AccessRead,
	"query show":                      AccessRead,
	"query sources":                   AccessRead,
	"query stats":                     AccessRead,
	"query tables":                    AccessRead,
	"admin reindex":                   AccessWrite,
	"admin rotate-serve-keys":         AccessWrite,
	"admin serve-keys":                AccessRead,
	"admin reindex-status":            AccessRead,
	"apikeys create":                  AccessWrite,
	"apikeys delete":                  AccessWrite,
	"apikeys list":                    AccessRead,
	"assets delete":                   AccessWrite,
	"assets get":                      AccessRead,
	"assets list":                     AccessRead,
	"assets owners":                   AccessRead,
	"assets search":                   AccessRead,
	"assets summary":                  AccessRead,
	"assets tags add":                 AccessWrite,
	"assets tags remove":              AccessWrite,
	"agent schema":                    AccessRead,
	"config get":                      AccessRead,
	"config init":                     AccessLocal,
	"config list":                     AccessRead,
	"config set":                      AccessLocal,
	"context delete":                  AccessLocal,
	"context list":                    AccessRead,
	"context use":                     AccessLocal,
	"credential-helper":               AccessLocal,
	"generate-encryption-key":         AccessRead,
	"glossary create":                 AccessWrite,
	"glossary delete":                 AccessWrite,
	"glossary get":                    AccessRead,
	"glossary list":                   AccessRead,
	"glossary search":                 AccessRead,
	"glossary update":                 AccessWrite,
	"ingest":                          AccessWrite,
	"lineage get":                     AccessRead,
	"login":                           AccessLocal,
	"logout":                          AccessLocal,
	"metrics by-provider":             AccessRead,
	"metrics by-type":                 AccessRead,
	"metrics summary":                 AccessRead,
	"metrics top-assets":              AccessRead,
	"metrics top-queries":             AccessRead,
	"operator":                        AccessWrite,
	"run":                             AccessWrite,
	"runs entities":                   AccessRead,
	"runs get":                        AccessRead,
	"runs list":                       AccessRead,
	"search":                          AccessRead,
	"skills install":                  AccessLocal,
	"service-accounts apikeys create": AccessWrite,
	"service-accounts apikeys delete": AccessWrite,
	"service-accounts apikeys list":   AccessRead,
	"service-accounts create":         AccessWrite,
	"service-accounts delete":         AccessWrite,
	"service-accounts get":            AccessRead,
	"service-accounts list":           AccessRead,
	"teams get":                       AccessRead,
	"teams list":                      AccessRead,
	"teams members":                   AccessRead,
	"users get":                       AccessRead,
	"users list":                      AccessRead,
	"users me":                        AccessRead,
	"version":                         AccessRead,
}

func commandKey(c *cobra.Command) string {
	return strings.TrimPrefix(strings.TrimPrefix(c.CommandPath(), c.Root().Name()), " ")
}

// accessOf returns a command's classification, treating an unclassified command as a write so read-only mode fails closed.
func accessOf(c *cobra.Command) Access {
	if a, ok := commandAccess[commandKey(c)]; ok {
		return a
	}
	return AccessWrite
}

// readOnly reports whether --read-only, MARMOT_READ_ONLY or read_only in the config file is set.
func readOnly() bool {
	return viper.GetBool("read_only")
}

// enforceReadOnly blocks every command that is not a read while read-only mode is on. Local configuration changes are blocked too, otherwise `config set read_only false` would switch the mode off. It is a guardrail for agents rather than a security boundary: the API key's permissions are the boundary.
func enforceReadOnly(cmd *cobra.Command, _ []string) error {
	if !cmd.Runnable() || isGroup(cmd) || !readOnly() {
		return nil
	}
	if access := accessOf(cmd); access != AccessRead {
		return withExitCode(ExitReadOnly, fmt.Errorf("%s is a %s command and read-only mode is on (--read-only, MARMOT_READ_ONLY or read_only in the config file)", cmd.CommandPath(), access))
	}
	return nil
}

var agentCmd = &cobra.Command{
	Use:   "agent",
	Short: "Help AI agents drive this CLI",
	Long: `Commands for AI agents and scripts that drive the Marmot CLI.

For agents, set MARMOT_READ_ONLY=true unless the agent is meant to change things, ask for
-o json, and branch on the exit code rather than the error message.`,
}

var agentSchemaCmd = &cobra.Command{
	Use:   "schema",
	Short: "Print every command, flag, exit code and setting as JSON",
	Long: `Prints the whole CLI as one JSON document: every command with its arguments, flags,
examples and whether it reads, writes to the server or changes local configuration, plus the
global flags, environment variables and exit codes. Agents can load it once instead of
walking --help.`,
	Example: `  marmot agent schema
  marmot agent schema | jq '.commands[] | select(.access == "read") | .command'`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(buildSchema(cmd.Root()))
	},
}

type schemaFlag struct {
	Name      string `json:"name"`
	Shorthand string `json:"shorthand,omitempty"`
	Type      string `json:"type"`
	Default   string `json:"default,omitempty"`
	Usage     string `json:"usage"`
	Required  bool   `json:"required,omitempty"`
}

type schemaCommand struct {
	Command  string       `json:"command"`
	Usage    string       `json:"usage"`
	Short    string       `json:"short"`
	Long     string       `json:"long,omitempty"`
	Aliases  []string     `json:"aliases,omitempty"`
	Access   Access       `json:"access"`
	Examples []string     `json:"examples,omitempty"`
	Flags    []schemaFlag `json:"flags,omitempty"`
}

type schemaEnv struct {
	Name    string `json:"name"`
	Meaning string `json:"meaning"`
}

type schemaDoc struct {
	Name          string          `json:"name"`
	Version       string          `json:"version"`
	OutputFormats []string        `json:"output_formats"`
	GlobalFlags   []schemaFlag    `json:"global_flags"`
	Environment   []schemaEnv     `json:"environment"`
	ExitCodes     any             `json:"exit_codes"`
	Commands      []schemaCommand `json:"commands"`
}

func buildSchema(root *cobra.Command) schemaDoc {
	doc := schemaDoc{
		Name:          root.Name(),
		Version:       Version,
		OutputFormats: []string{"table", "json", "yaml"},
		GlobalFlags:   schemaFlags(root.PersistentFlags()),
		Environment: []schemaEnv{
			{"MARMOT_HOST", "Marmot API host, as --host."},
			{"MARMOT_API_KEY", "API key, as --api-key."},
			{"MARMOT_OUTPUT", "Default output format, as --output."},
			{"MARMOT_READ_ONLY", "When true, every command that is not a read exits with code 9, as --read-only."},
		},
		ExitCodes: exitCodeDocs,
	}
	var walk func(c *cobra.Command)
	walk = func(c *cobra.Command) {
		if c.Runnable() && !isGroup(c) && !c.Hidden && c.Name() != "help" && c.Name() != "completion" {
			doc.Commands = append(doc.Commands, schemaCommand{
				Command:  c.CommandPath(),
				Usage:    c.UseLine(),
				Short:    c.Short,
				Long:     c.Long,
				Aliases:  c.Aliases,
				Access:   accessOf(c),
				Examples: splitExamples(c.Example),
				Flags:    schemaFlags(c.LocalNonPersistentFlags()),
			})
		}
		for _, sub := range c.Commands() {
			walk(sub)
		}
	}
	walk(root)
	sort.Slice(doc.Commands, func(i, j int) bool { return doc.Commands[i].Command < doc.Commands[j].Command })
	return doc
}

func schemaFlags(fs *pflag.FlagSet) []schemaFlag {
	var out []schemaFlag
	fs.VisitAll(func(f *pflag.Flag) {
		if f.Hidden || f.Name == "help" {
			return
		}
		_, required := f.Annotations[cobra.BashCompOneRequiredFlag]
		def := f.DefValue
		if def == "[]" || (f.Value.Type() == "bool" && def == "false") {
			def = ""
		}
		out = append(out, schemaFlag{
			Name:      f.Name,
			Shorthand: f.Shorthand,
			Type:      f.Value.Type(),
			Default:   def,
			Usage:     f.Usage,
			Required:  required,
		})
	})
	return out
}

func splitExamples(example string) []string {
	var out []string
	for _, line := range strings.Split(example, "\n") {
		if line = strings.TrimSpace(line); line != "" {
			out = append(out, line)
		}
	}
	return out
}

func init() {
	agentCmd.AddCommand(agentSchemaCmd)
	rootCmd.AddCommand(agentCmd)
}
