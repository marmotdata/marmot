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
