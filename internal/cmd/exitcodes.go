package cmd

import (
	"bufio"
	"errors"
	"fmt"
	"net"
	"net/url"
	"os"
	"strings"

	marmot "github.com/marmotdata/marmot/sdk/go"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

// Exit codes are part of the CLI's contract with scripts and agents, so a caller can branch on the kind of failure without parsing the message; never renumber them.
const (
	ExitOK                   = 0
	ExitError                = 1
	ExitUsage                = 2
	ExitUnauthenticated      = 3
	ExitForbidden            = 4
	ExitNotFound             = 5
	ExitInvalid              = 6
	ExitRateLimited          = 7
	ExitServer               = 8
	ExitReadOnly             = 9
	ExitConfirmationRequired = 10
	ExitCancelled            = 11
)

// exitCodeDocs describes each code for `marmot agent schema` and the help text.
var exitCodeDocs = []struct {
	Code    int    `json:"code"`
	Name    string `json:"name"`
	Meaning string `json:"meaning"`
}{
	{ExitOK, "ok", "The command succeeded."},
	{ExitError, "error", "Any failure not covered below."},
	{ExitUsage, "usage", "Wrong arguments or flags; fix the command line and retry."},
	{ExitUnauthenticated, "unauthenticated", "No valid credentials; run marmot login or set MARMOT_API_KEY."},
	{ExitForbidden, "forbidden", "Authenticated but not allowed, including a data query you may not run; retrying will not help."},
	{ExitNotFound, "not_found", "The resource does not exist or is not visible to you."},
	{ExitInvalid, "invalid", "The server rejected the request as invalid."},
	{ExitRateLimited, "rate_limited", "Too many requests; wait and retry."},
	{ExitServer, "server", "The server failed or could not be reached; retrying later may help."},
	{ExitReadOnly, "read_only", "The command changes something and read-only mode is on."},
	{ExitConfirmationRequired, "confirmation_required", "The command needs confirmation and there is no terminal to ask on; pass --yes once the change is agreed."},
	{ExitCancelled, "cancelled", "The confirmation prompt was declined."},
}

// exitError carries an exit code alongside the message cobra prints.
type exitError struct {
	code int
	err  error
}

func (e *exitError) Error() string { return e.err.Error() }
func (e *exitError) Unwrap() error { return e.err }

func withExitCode(code int, err error) error {
	if err == nil {
		return nil
	}
	return &exitError{code: code, err: err}
}

// usageError reports a command line mistake.
func usageError(format string, args ...any) error {
	return withExitCode(ExitUsage, fmt.Errorf(format, args...))
}

// ExitCode maps an error returned by Execute to the process exit code.
func ExitCode(err error) int {
	if err == nil {
		return ExitOK
	}
	var coded *exitError
	if errors.As(err, &coded) {
		return coded.code
	}
	var apiErr *marmot.APIError
	if errors.As(err, &apiErr) {
		return exitCodeForStatus(apiErr.StatusCode)
	}
	var (
		authErr       *marmot.AuthError
		notFoundErr   *marmot.NotFoundError
		validationErr *marmot.ValidationError
		rateErr       *marmot.RateLimitError
		serverErr     *marmot.ServerError
	)
	switch {
	case errors.As(err, &authErr):
		return exitCodeForStatus(authErr.StatusCode)
	case errors.As(err, &notFoundErr):
		return ExitNotFound
	case errors.As(err, &validationErr):
		return ExitInvalid
	case errors.As(err, &rateErr):
		return ExitRateLimited
	case errors.As(err, &serverErr):
		return ExitServer
	}
	var netErr net.Error
	var urlErr *url.Error
	if errors.As(err, &netErr) || errors.As(err, &urlErr) {
		return ExitServer
	}
	return ExitError
}

func exitCodeForStatus(status int) int {
	switch {
	case status == 401:
		return ExitUnauthenticated
	case status == 403:
		return ExitForbidden
	case status == 404:
		return ExitNotFound
	case status == 429:
		return ExitRateLimited
	case status >= 500:
		return ExitServer
	case status >= 400:
		return ExitInvalid
	}
	return ExitError
}

// groupAnnotation marks a command that only groups subcommands, which read-only mode and the schema ignore.
const groupAnnotation = "marmot.io/group"

func isGroup(c *cobra.Command) bool {
	return c.Annotations[groupAnnotation] == "true" || (!c.Runnable() && c.HasSubCommands())
}

// markUsageErrors makes flag and argument mistakes exit with ExitUsage, which cobra otherwise reports as plain errors.
func markUsageErrors(root *cobra.Command) {
	root.SetFlagErrorFunc(func(_ *cobra.Command, err error) error {
		return withExitCode(ExitUsage, err)
	})
	var walk func(c *cobra.Command)
	walk = func(c *cobra.Command) {
		// A group such as `marmot query` prints its help and exits 0 when given an unknown subcommand, so make that a usage error.
		if !c.Runnable() && c.HasSubCommands() && c != root {
			if c.Annotations == nil {
				c.Annotations = map[string]string{}
			}
			c.Annotations[groupAnnotation] = "true"
			c.Args = cobra.ArbitraryArgs
			c.RunE = func(cmd *cobra.Command, a []string) error {
				if len(a) > 0 {
					return usageError("unknown command %q for %q", a[0], cmd.CommandPath())
				}
				return cmd.Help()
			}
		}
		if args := c.Args; args != nil {
			c.Args = func(cmd *cobra.Command, a []string) error {
				return withExitCode(ExitUsage, args(cmd, a))
			}
		}
		for _, sub := range c.Commands() {
			walk(sub)
		}
	}
	walk(root)
}

// confirm asks on stderr, so stdout stays clean for -o json. Without a
// terminal to ask on it refuses rather than guessing; --yes answers for you.
func confirm(cmd *cobra.Command, question string) error {
	if yes, _ := cmd.Flags().GetBool("yes"); yes {
		return nil
	}
	if !term.IsTerminal(int(os.Stdin.Fd())) {
		return withExitCode(ExitConfirmationRequired, fmt.Errorf("%s needs confirmation and there is no terminal to ask on; pass --yes", cmd.CommandPath()))
	}
	fmt.Fprintf(os.Stderr, "%s (y/N): ", question)
	answer, _ := bufio.NewReader(os.Stdin).ReadString('\n')
	answer = strings.ToLower(strings.TrimSpace(answer))
	if answer != "y" && answer != "yes" {
		return withExitCode(ExitCancelled, errors.New("cancelled"))
	}
	return nil
}
