package cmd

import (
	"errors"
	"testing"

	marmot "github.com/marmotdata/marmot/sdk/go"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEveryCommandHasAccess(t *testing.T) {
	seen := map[string]bool{}
	var walk func(c *cobra.Command)
	walk = func(c *cobra.Command) {
		if c.Runnable() && !isGroup(c) && c != rootCmd && c.Name() != "help" && c.Name() != "completion" {
			key := commandKey(c)
			seen[key] = true
			_, ok := commandAccess[key]
			assert.True(t, ok, "classify %q in commandAccess so read-only mode knows whether it writes", key)
		}
		for _, sub := range c.Commands() {
			walk(sub)
		}
	}
	walk(rootCmd)
	for key := range commandAccess {
		assert.True(t, seen[key], "commandAccess lists %q, which is not a command", key)
	}
}

func TestReadOnlyBlocksWrites(t *testing.T) {
	viper.Set("read_only", true)
	t.Cleanup(func() { viper.Set("read_only", false) })

	assert.NoError(t, enforceReadOnly(queryRunCmd, nil))
	err := enforceReadOnly(glossaryCreateCmd, nil)
	require.Error(t, err)
	assert.Equal(t, ExitReadOnly, ExitCode(err))
	assert.Equal(t, ExitReadOnly, ExitCode(enforceReadOnly(configSetCmd, nil)))

	viper.Set("read_only", false)
	assert.NoError(t, enforceReadOnly(glossaryCreateCmd, nil))
}

func TestExitCode(t *testing.T) {
	api := func(status int) error { return &marmot.APIError{StatusCode: status, Message: "x"} }
	cases := []struct {
		err  error
		want int
	}{
		{nil, ExitOK},
		{errors.New("boom"), ExitError},
		{usageError("bad"), ExitUsage},
		{&marmot.AuthError{APIError: &marmot.APIError{StatusCode: 401}}, ExitUnauthenticated},
		{&marmot.AuthError{APIError: &marmot.APIError{StatusCode: 403}}, ExitForbidden},
		{&marmot.NotFoundError{APIError: &marmot.APIError{StatusCode: 404}}, ExitNotFound},
		{&marmot.ValidationError{APIError: &marmot.APIError{StatusCode: 400}}, ExitInvalid},
		{&marmot.RateLimitError{APIError: &marmot.APIError{StatusCode: 429}}, ExitRateLimited},
		{&marmot.ServerError{APIError: &marmot.APIError{StatusCode: 502}}, ExitServer},
		{api(409), ExitInvalid},
		{withExitCode(ExitCancelled, errors.New("cancelled")), ExitCancelled},
	}
	for _, c := range cases {
		assert.Equal(t, c.want, ExitCode(c.err), "%v", c.err)
	}
}

func TestSchemaCoversCommands(t *testing.T) {
	doc := buildSchema(rootCmd)
	byName := map[string]schemaCommand{}
	for _, c := range doc.Commands {
		byName[c.Command] = c
	}
	query, ok := byName["marmot query run"]
	require.True(t, ok)
	assert.Equal(t, AccessRead, query.Access)
	assert.NotEmpty(t, query.Examples)
	assert.Equal(t, AccessWrite, byName["marmot glossary create"].Access)
	assert.Len(t, doc.ExitCodes, 12)
}
