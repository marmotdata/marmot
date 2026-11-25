package github

import (
	"context"
	"errors"
	"fmt"
	"math"
	"net/http"
	"regexp"
	"slices"

	pluginsdk "github.com/marmotdata/plugin-sdk"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// ToolsConfig bounds what the plugin's tools may change, on top of what
// the token itself may do. It is the runner's own config, so a caller
// choosing a tool cannot widen it.
type ToolsConfig struct {
	Repos          []string `json:"repos,omitempty" description:"Repositories, by name, that tools may change. Empty allows none; reads need no entry here"`
	BranchPrefix   string   `json:"branch_prefix,omitempty" description:"Prefix of every branch the plugin creates, ending in a slash. Only pull requests from such a branch can be closed or reopened" default:"marmot/"`
	MaxFiles       int      `json:"max_files,omitempty" description:"Most files one pull request may change, from 1 to 3" default:"3"`
	MaxFileBytes   int      `json:"max_file_bytes,omitempty" description:"Largest file, in bytes, a pull request may edit, up to 1 MiB" default:"524288"`
	AllowCIChanges bool     `json:"allow_ci_changes,omitempty" label:"Allow CI changes" description:"Allow pull requests that change .github/workflows or .github/actions. A pull request from a branch in the repository runs its own workflow changes, with the repository's secrets"`
}

const (
	defaultBranchPrefix = "marmot/"
	// maxEdits is the manifest's number of edit slots, which is static so
	// its digest is too; tools.max_files narrows it per runner.
	maxEdits            = 3
	defaultMaxFileBytes = 512 << 10
	maxFileBytesCeiling = 1 << 20
)

var branchPrefixPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]{0,29}/$`)

// Validate checks the tools settings.
func (c *ToolsConfig) Validate() error {
	for _, r := range c.Repos {
		if !validRepoName(r) {
			return fmt.Errorf("tools.repos: %q is not a repository name", r)
		}
	}
	if c.BranchPrefix != "" && !branchPrefixPattern.MatchString(c.BranchPrefix) {
		return fmt.Errorf("tools.branch_prefix %q must be lower case, at most 31 characters, and end in a slash, such as marmot/", c.BranchPrefix)
	}
	if c.MaxFiles < 0 || c.MaxFiles > maxEdits {
		return fmt.Errorf("tools.max_files must be between 1 and %d", maxEdits)
	}
	if c.MaxFileBytes < 0 || c.MaxFileBytes > maxFileBytesCeiling {
		return fmt.Errorf("tools.max_file_bytes must be between 1 and %d", maxFileBytesCeiling)
	}
	return nil
}

// The nested tools section gets no defaults from the SDK, so zero means
// the default here.
func (c *ToolsConfig) branchPrefix() string {
	if c.BranchPrefix == "" {
		return defaultBranchPrefix
	}
	return c.BranchPrefix
}

func (c *ToolsConfig) maxFiles() int {
	if c.MaxFiles == 0 {
		return maxEdits
	}
	return c.MaxFiles
}

func (c *ToolsConfig) maxFileBytes() int64 {
	if c.MaxFileBytes == 0 {
		return defaultMaxFileBytes
	}
	return int64(c.MaxFileBytes)
}

func (c *ToolsConfig) writable(repo string) bool {
	return slices.Contains(c.Repos, repo)
}

// Target kinds, for policy to scope tools by what they touch.
const (
	kindPullRequest pluginsdk.TargetKind = "github/pullrequest"
	kindComment     pluginsdk.TargetKind = "github/comment"
	kindFile        pluginsdk.TargetKind = "github/file"
	kindRepository  pluginsdk.TargetKind = "github/repository"
)

// Tool names.
const (
	toolOpenPullRequest   = "open_pull_request"
	toolClosePullRequest  = "close_pull_request"
	toolReopenPullRequest = "reopen_pull_request"
	toolComment           = "comment_on_pull_request"
	toolDeleteComment     = "delete_comment"
)

const (
	// pathChars bounds a path parameter to characters that need no quoting
	// anywhere it is shown; traversal and .git are refused in code.
	pathChars = `[A-Za-z0-9._@+/-]+`
	// refChars is the same for a branch, tag or commit.
	refChars = `[A-Za-z0-9._/-]+`
)

var (
	repoParam = pluginsdk.ToolParam{
		Name: "repo", Description: "Repository name, under the configured owner", Type: pluginsdk.ParamString,
		Required: true, MaxLength: 100, Pattern: `[A-Za-z0-9._-]+`,
	}
	numberParam = pluginsdk.ToolParam{
		Name: "number", Description: "Pull request number", Type: pluginsdk.ParamInt, Required: true, Min: 1, Max: math.MaxInt32,
	}
	refParam = pluginsdk.ToolParam{
		Name: "ref", Description: "Branch, tag or commit to read. Defaults to the default branch", Type: pluginsdk.ParamString,
		MaxLength: 255, Pattern: refChars,
	}
)

// ToolSpecs is the plugin's tool manifest. The writes open, close and
// reopen pull requests from the plugin's own branches and comment on pull
// requests: none merges, pushes to an existing branch, deletes a branch it
// did not just create, or touches repository settings. The reads, at the
// end, return files and pull requests and change nothing.
func ToolSpecs() []pluginsdk.ToolSpec {
	return append(writeSpecs(), readSpecs()...)
}

func editParams() []pluginsdk.ToolParam {
	var params []pluginsdk.ToolParam
	for i := 1; i <= maxEdits; i++ {
		params = append(params,
			pluginsdk.ToolParam{
				Name: fmt.Sprintf("path_%d", i), Description: fmt.Sprintf("Path of the file edit %d changes, which must exist", i),
				Type: pluginsdk.ParamString, Required: i == 1, MaxLength: 512, Pattern: pathChars,
			},
			pluginsdk.ToolParam{
				Name: fmt.Sprintf("old_%d", i), Description: fmt.Sprintf("Text edit %d replaces, which must occur exactly once in the file. Parameters cannot carry a line break, so an edit stays within one line", i),
				Type: pluginsdk.ParamString, Required: i == 1, MaxLength: pluginsdk.MaxStringParamLength,
			},
			pluginsdk.ToolParam{
				Name: fmt.Sprintf("new_%d", i), Description: fmt.Sprintf("Text that replaces old_%d. Empty removes it", i),
				Type: pluginsdk.ParamString, Required: i == 1, MaxLength: pluginsdk.MaxStringParamLength,
			},
		)
	}
	return params
}

func writeSpecs() []pluginsdk.ToolSpec {
	return []pluginsdk.ToolSpec{
		{
			Name:    toolOpenPullRequest,
			Version: 1,
			Title:   "Open a pull request",
			Description: "Creates a branch under the runner's branch prefix from the base branch, commits up to three single-line text edits to existing files in one commit, and opens a pull request. Nothing is merged. " +
				"Changes to .github/workflows and .github/actions are refused unless the runner allows CI changes. Undone by closing the pull request.",
			Targets: []pluginsdk.TargetKind{kindPullRequest},
			Params: append([]pluginsdk.ToolParam{
				repoParam,
				{Name: "base", Description: "Branch to start from and merge into. Defaults to the default branch", Type: pluginsdk.ParamString, MaxLength: 255, Pattern: refChars},
				{Name: "branch", Description: "Name of the new branch after the runner's prefix. Defaults to one made from the title", Type: pluginsdk.ParamString, MaxLength: 60, Pattern: `[a-z0-9][a-z0-9._-]*`},
				{Name: "title", Description: "Pull request title, also the commit message", Type: pluginsdk.ParamString, Required: true, MaxLength: 256},
				{Name: "body", Description: "Pull request description", Type: pluginsdk.ParamString, MaxLength: pluginsdk.MaxStringParamLength},
			}, editParams()...),
			Reversibility: pluginsdk.ReversibilityReversible,
			// Nothing reaches the base branch without a person merging it,
			// so it is bounded, though CI may run on the new branch.
			Risk:    pluginsdk.RiskBounded,
			Inverse: toolClosePullRequest,
		},
		{
			Name:          toolClosePullRequest,
			Version:       1,
			Title:         "Close a pull request",
			Description:   "Closes an open, unmerged pull request from one of the plugin's own branches in the same repository. The branch is kept, so the pull request can be reopened.",
			Targets:       []pluginsdk.TargetKind{kindPullRequest},
			Params:        []pluginsdk.ToolParam{repoParam, numberParam},
			Reversibility: pluginsdk.ReversibilityReversible,
			Risk:          pluginsdk.RiskBounded,
			Inverse:       toolReopenPullRequest,
		},
		{
			// Only here because close_pull_request is reversible, and a
			// reversible tool's inverse must be in the manifest.
			Name:          toolReopenPullRequest,
			Version:       1,
			Title:         "Reopen a pull request",
			Description:   "Reopens a closed, unmerged pull request from one of the plugin's own branches, if the branch still exists.",
			Targets:       []pluginsdk.TargetKind{kindPullRequest},
			Params:        []pluginsdk.ToolParam{repoParam, numberParam},
			Reversibility: pluginsdk.ReversibilityReversible,
			Risk:          pluginsdk.RiskBounded,
			Inverse:       toolClosePullRequest,
		},
		{
			Name:        toolComment,
			Version:     1,
			Title:       "Comment on a pull request",
			Description: "Adds a comment to an open pull request. Undone by deleting the comment, though notifications already sent stay sent.",
			Targets:     []pluginsdk.TargetKind{kindPullRequest},
			Params: []pluginsdk.ToolParam{
				repoParam, numberParam,
				{Name: "body", Description: "Comment text", Type: pluginsdk.ParamString, Required: true, MaxLength: pluginsdk.MaxStringParamLength},
			},
			// Reversible rather than irreversible: delete_comment removes
			// the comment and everything it says, which is the change. The
			// e-mails GitHub sends when it is posted cannot be recalled;
			// the plan warns of that instead of the tier pretending a
			// comment cannot be taken back.
			Reversibility: pluginsdk.ReversibilityReversible,
			Risk:          pluginsdk.RiskBounded,
			Inverse:       toolDeleteComment,
		},
		{
			// Only here as comment_on_pull_request's inverse. It deletes
			// nothing but a comment the plugin posted.
			Name:        toolDeleteComment,
			Version:     1,
			Title:       "Delete a comment",
			Description: "Deletes a pull request comment the plugin posted, recognised by the marker it adds to each. Undone by posting the same text again, as a new comment.",
			Targets:     []pluginsdk.TargetKind{kindComment},
			Params: []pluginsdk.ToolParam{
				repoParam,
				{Name: "comment_id", Description: "Comment id", Type: pluginsdk.ParamInt, Required: true, Min: 1, Max: 1<<53 - 1},
			},
			Reversibility: pluginsdk.ReversibilityReversible,
			Risk:          pluginsdk.RiskBounded,
			Inverse:       toolComment,
		},
	}
}

// Tools returns the plugin's tool manifest.
func (s *Source) Tools() []pluginsdk.ToolSpec {
	return ToolSpecs()
}

// PlanTool previews a write without changing anything.
func (s *Source) PlanTool(ctx context.Context, rawConfig pluginsdk.RawConfig, req pluginsdk.ToolRequest) (*pluginsdk.ToolPlan, error) {
	engine, err := newEngine(rawConfig, req.Origin)
	if err != nil {
		return nil, err
	}
	plan, err := engine.Plan(ctx, req)
	return plan, wireError(err)
}

// ExecuteTool runs a read, or a write against the state its plan saw.
func (s *Source) ExecuteTool(ctx context.Context, rawConfig pluginsdk.RawConfig, req pluginsdk.ToolRequest) (*pluginsdk.ToolResult, error) {
	engine, err := newEngine(rawConfig, req.Origin)
	if err != nil {
		return nil, err
	}
	result, err := engine.Execute(ctx, req)
	return result, wireError(err)
}

var (
	// ErrStale is returned when the target changed after it was planned.
	ErrStale = errors.New("changed since the tool was planned; plan it again")
	// ErrOutOfScope is returned for a repository or object the runner's
	// config does not let tools touch.
	ErrOutOfScope = errors.New("outside the tools scope")
	// ErrNotOurs is returned for a pull request or comment the plugin did
	// not make.
	ErrNotOurs = errors.New("not made by this plugin")
	// ErrCIPath refuses an edit to the repository's CI definitions.
	ErrCIPath = errors.New("a CI definition")
	// ErrBadPath refuses a path that could leave the repository's tree or
	// reach into .git.
	ErrBadPath = errors.New("not a plain path inside the repository")
)

// wireError gives a refusal the gRPC code that says what to do about it, so
// Marmot can tell "plan it again" from "not allowed here" from a fault.
func wireError(err error) error {
	if err == nil {
		return nil
	}
	if _, ok := status.FromError(err); ok {
		return err
	}
	switch {
	case errors.Is(err, ErrStale):
		return status.Error(codes.FailedPrecondition, err.Error())
	case errors.Is(err, ErrOutOfScope), errors.Is(err, ErrNotOurs), errors.Is(err, ErrCIPath):
		return status.Error(codes.PermissionDenied, err.Error())
	case errors.Is(err, ErrBadPath):
		return status.Error(codes.InvalidArgument, err.Error())
	}
	switch statusOf(err) {
	case http.StatusUnauthorized, http.StatusForbidden:
		return status.Error(codes.PermissionDenied, err.Error())
	case http.StatusNotFound:
		return status.Error(codes.NotFound, err.Error())
	case http.StatusConflict, http.StatusUnprocessableEntity:
		return status.Error(codes.FailedPrecondition, err.Error())
	}
	return err
}
