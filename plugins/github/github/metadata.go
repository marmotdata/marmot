package github

// RepositoryFields represents GitHub repository metadata fields
// +marmot:metadata
type RepositoryFields struct {
	Owner         string   `json:"owner" metadata:"owner" description:"Organisation or user that owns the repository"`
	Repository    string   `json:"repository" metadata:"repository" description:"Repository name"`
	FullName      string   `json:"full_name" metadata:"full_name" description:"Owner and name (owner/repository)"`
	DefaultBranch string   `json:"default_branch" metadata:"default_branch" description:"Default branch"`
	Language      string   `json:"language" metadata:"language" description:"Primary language, as GitHub detects it"`
	Visibility    string   `json:"visibility" metadata:"visibility" description:"Visibility (public, private, internal)"`
	Archived      bool     `json:"archived" metadata:"archived" description:"Whether the repository is archived and read-only"`
	Fork          bool     `json:"fork" metadata:"fork" description:"Whether the repository is a fork"`
	Topics        []string `json:"topics" metadata:"topics" description:"Repository topics"`
	URL           string   `json:"url" metadata:"url" description:"Repository web URL"`
	License       string   `json:"license" metadata:"license" description:"SPDX identifier of the detected license"`
	Stars         int      `json:"stars" metadata:"stars" description:"Number of stargazers"`
	Forks         int      `json:"forks" metadata:"forks" description:"Number of forks"`
	OpenIssues    int      `json:"open_issues" metadata:"open_issues" description:"Open issues and pull requests"`
	CreatedAt     string   `json:"created_at" metadata:"created_at" description:"When the repository was created"`
	PushedAt      string   `json:"pushed_at" metadata:"pushed_at" description:"When anything was last pushed to the repository"`
	Owners        []string `json:"owners" metadata:"owners" description:"Default code owners, from the catch-all rule in CODEOWNERS"`
}

// BranchFields represents GitHub branch metadata fields
// +marmot:metadata
type BranchFields struct {
	Repository string `json:"repository" metadata:"repository" description:"Owner and name of the repository (owner/repository)"`
	Branch     string `json:"branch" metadata:"branch" description:"Branch name"`
	Default    bool   `json:"default" metadata:"default" description:"Whether this is the repository's default branch"`
	CommitSHA  string `json:"commit_sha" metadata:"commit_sha" description:"Commit the branch points at"`
	URL        string `json:"url" metadata:"url" description:"Branch web URL"`
}

// PullRequestFields represents GitHub pull request metadata fields
// +marmot:metadata
type PullRequestFields struct {
	Repository string   `json:"repository" metadata:"repository" description:"Owner and name of the repository (owner/repository)"`
	Number     int      `json:"number" metadata:"number" description:"Pull request number"`
	Title      string   `json:"title" metadata:"title" description:"Title"`
	Author     string   `json:"author" metadata:"author" description:"Login of who opened it"`
	Draft      bool     `json:"draft" metadata:"draft" description:"Whether it is a draft"`
	HeadBranch string   `json:"head_branch" metadata:"head_branch" description:"Branch with the changes"`
	BaseBranch string   `json:"base_branch" metadata:"base_branch" description:"Branch it merges into"`
	Labels     []string `json:"labels" metadata:"labels" description:"Labels"`
	Assignees  []string `json:"assignees" metadata:"assignees" description:"Logins of assignees"`
	Reviewers  []string `json:"reviewers" metadata:"reviewers" description:"Logins of requested reviewers"`
	URL        string   `json:"url" metadata:"url" description:"Pull request web URL"`
	CreatedAt  string   `json:"created_at" metadata:"created_at" description:"When it was opened"`
	UpdatedAt  string   `json:"updated_at" metadata:"updated_at" description:"When it was last updated"`
}

// IssueFields represents GitHub issue metadata fields
// +marmot:metadata
type IssueFields struct {
	Repository string   `json:"repository" metadata:"repository" description:"Owner and name of the repository (owner/repository)"`
	Number     int      `json:"number" metadata:"number" description:"Issue number"`
	Title      string   `json:"title" metadata:"title" description:"Title"`
	Author     string   `json:"author" metadata:"author" description:"Login of who opened it"`
	Labels     []string `json:"labels" metadata:"labels" description:"Labels"`
	Assignees  []string `json:"assignees" metadata:"assignees" description:"Logins of assignees"`
	Comments   int      `json:"comments" metadata:"comments" description:"Number of comments"`
	URL        string   `json:"url" metadata:"url" description:"Issue web URL"`
	CreatedAt  string   `json:"created_at" metadata:"created_at" description:"When it was opened"`
	UpdatedAt  string   `json:"updated_at" metadata:"updated_at" description:"When it was last updated"`
}
