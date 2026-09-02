package cmd

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"
)

// credentialHelperCmd implements the Docker credential helper protocol: the
// operation as an argument, input on stdin, JSON on stdout. Docker, crane and
// oras run it as docker-credential-marmot; main routes that name here. It is
// hidden because nobody types it.
var credentialHelperCmd = &cobra.Command{
	Use:    "credential-helper <get|store|erase|list>",
	Short:  "Docker credential helper backed by marmot login",
	Hidden: true,
	// The protocol reports failures on stdout, and that is all Docker reads.
	SilenceUsage:  true,
	SilenceErrors: true,
	Args:          cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return runCredentialHelper(args[0], cmd.InOrStdin(), cmd.OutOrStdout())
	},
}

func init() {
	rootCmd.AddCommand(credentialHelperCmd)
}

// errCredentialsNotFound is the exact text Docker treats as "no credentials,
// continue anonymously" rather than as a broken helper.
var errCredentialsNotFound = errors.New("credentials not found in native keychain")

// errHelperFailed means the failure text was already written to stdout.
var errHelperFailed = errors.New("credential helper failed")

type helperCredential struct {
	ServerURL string
	Username  string
	Secret    string
}

func runCredentialHelper(op string, in io.Reader, out io.Writer) error {
	fail := func(msg string) error {
		fmt.Fprintln(out, msg)
		return errHelperFailed
	}

	input, err := io.ReadAll(in)
	if err != nil {
		return fail("reading stdin: " + err.Error())
	}
	server := strings.TrimSpace(string(input))

	switch op {
	case "get":
		name, ok := contextForRegistry(server)
		if !ok {
			return fail(errCredentialsNotFound.Error())
		}
		token, ok := getCachedToken(name)
		if !ok {
			return fail(fmt.Sprintf("marmot: not logged in to %s or the token has expired, run: marmot login %s", name, name))
		}
		return json.NewEncoder(out).Encode(helperCredential{
			ServerURL: server,
			Username:  registryUsername,
			Secret:    token,
		})
	case "store":
		// Only marmot login writes tokens. Otherwise docker login would
		// replace one with a password the registry cannot check.
		return fail("marmot: credentials for this registry are managed by `marmot login`")
	case "erase":
		if name, ok := contextForRegistry(server); ok {
			return deleteCachedToken(name)
		}
		return nil
	case "list":
		return json.NewEncoder(out).Encode(helperList())
	}
	return fail(fmt.Sprintf("unknown credential helper operation %q", op))
}

// contextForRegistry finds the saved context whose host serves the registry.
func contextForRegistry(server string) (string, bool) {
	want := registryHost(server)
	if want == "" {
		return "", false
	}
	for name, ctx := range getContexts() {
		if registryHost(ctx.Host) == want {
			return name, true
		}
	}
	return "", false
}

// helperList maps every registry with a live token to its user name.
func helperList() map[string]string {
	out := map[string]string{}
	for name, ctx := range getContexts() {
		if _, ok := getCachedToken(name); ok {
			out[registryHost(ctx.Host)] = registryUsername
		}
	}
	return out
}
