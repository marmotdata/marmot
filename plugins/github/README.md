The GitHub plugin catalogues the repositories of a GitHub organisation or user. Each repository becomes an asset with its description, default branch, language, visibility, licence and URL, its topics as tags and, when it has one, its README as documentation. Discovery can be narrowed to named repositories or to those with given topics, and leaves private, internal and archived repositories out unless asked.

Default owners can be read from CODEOWNERS: the owners of its catch-all `*` rule, which is who GitHub asks to review a file no later rule claims. It costs up to three requests per repository, so it is off by default.

The plugin draws no lineage. A repository that holds a dbt project or a Dockerfile does not say which tables or images it produces, and guessing would put edges in the catalog that nothing checked. The plugins for the systems themselves (dbt, Airflow, Kubernetes) know what reads and writes what.

## Prerequisites

The plugin needs a token. Prefer a [fine-grained personal access token](https://docs.github.com/en/authentication/keeping-your-account-and-data-secure/managing-your-personal-access-tokens) or a GitHub App installation token, limited to the repositories you catalogue, and grant only the permissions for what you use:

| Capability | Repository permissions |
| --- | --- |
| Discovery, `read_file` and `list_files` | Metadata: read, Contents: read |
| `list_pull_requests` | Pull requests: read |
| `open_pull_request` | Contents: write, Pull requests: write |
| `close_pull_request`, `reopen_pull_request` | Pull requests: write |
| `comment_on_pull_request`, `delete_comment` | Pull requests: write |
| Any of the above with `actions.allow_ci_changes` | Workflows: write as well |

A classic token's `repo` scope grants all of this and much more besides, including every repository its user can reach, so avoid it.

:::tip[Authentication]
The token is read from `token` in the config, or from `$GITHUB_TOKEN` in the plugin's environment when `token` is unset. It is sent only to `base_url`, never logged, and never followed across a redirect.

A GitHub App installation token expires after an hour, so whatever starts the plugin must mint a fresh one. A fine-grained token lasts as long as you set.
:::

:::tip[GitHub Enterprise Server]
Set `base_url` to `https://<host>/api/v3`. Plain `http` is refused except on a loopback address.
:::

```yaml
owner: "acme"
token: "${GITHUB_TOKEN}"
topics:
  - "data"
include_private: true
include_codeowners: true
tags:
  - "github"
  - "${language}"
```

## Actions

Besides discovery, the plugin can propose changes, which a runner allowed to take actions runs on your behalf. It works through pull requests: it never merges, never pushes to a branch it did not just create, never deletes a branch other than its own and never touches repository settings. Every write is previewed first, and runs only if what it acts on is still exactly as previewed.

| Action | Changes | Undone by |
| --- | --- | --- |
| `open_pull_request` | Creates a branch from `base`, commits up to three edits in one commit and opens a pull request. Refused if `base` has moved since the plan | `close_pull_request` |
| `close_pull_request` | Closes an open pull request from one of the plugin's branches. The branch is kept | `reopen_pull_request` |
| `reopen_pull_request` | Reopens one, if its branch still exists | `close_pull_request` |
| `comment_on_pull_request` | Adds a comment to an open pull request. Refused if the pull request has new commits since the plan | `delete_comment` |
| `delete_comment` | Deletes a comment the plugin posted | `comment_on_pull_request`, as a new comment |

Each edit replaces text that occurs exactly once in an existing file with new text, and the plan shows the change as a unified diff. Action parameters cannot hold a line break, so an edit stays within one line: bumping a version, changing a setting or fixing a typo. The plugin cannot create, delete or rename files, and refuses binary files, symbolic links and submodules.

A pull request is the plugin's own when its branch starts with `actions.branch_prefix` in the same repository; a fork's branch never counts. Commits carry `Marmot-Origin` and `Marmot-Actor` trailers, and pull requests and comments end with a line naming who asked and from where. A comment also carries a hidden marker, and `delete_comment` refuses any comment without it or written by another account.

Changes to `.github/workflows` and `.github/actions` are refused. A pull request from a branch in the repository runs its own version of the workflows, with the repository's secrets, before anyone reviews it, so letting an action change them would let it change what CI does with those secrets. Set `actions.allow_ci_changes` only if you accept that. Workflows on `pull_request` still run against every branch the plugin creates, and run whatever scripts the branch holds, so keep deployment secrets in [environments](https://docs.github.com/en/actions/deployment/targeting-different-environments/using-environments-for-deployment) that require review.

Writes need the repository in `actions.repos`; without it they are refused before the repository is read:

```yaml
actions:
  repos:
    - "pipelines"
  branch_prefix: "marmot/"
  max_files: 3
  max_file_bytes: 524288
  allow_ci_changes: false
```

`max_files` is at most 3 and `max_file_bytes`, the largest file an edit may touch, at most 1 MiB. Branch protection on `base` still applies when someone merges.

### Reads

Three actions only read, so a task can look before it proposes a change. They need no plan and no approval, but the runner's policy must still name each one. They reach only the repositories discovery would catalogue and need no entry in `actions.repos`.

| Read | Returns |
| --- | --- |
| `read_file` | Up to 64 KiB of a text file at a branch, tag or commit, with its blob id and size. Binary files are reported, not returned |
| `list_files` | Up to 500 file paths and sizes under a directory, recursively |
| `list_pull_requests` | Up to 50 of the most recently updated open or closed pull requests, with branches, head commit, author and whether the plugin opened them |

File contents and pull request titles are written by whoever can push or open a pull request, so every read is marked untrusted. Nothing is redacted: a secret committed to a repository is returned like any other text.

### Runner policy

The runner's policy names each read and action it serves, and anything it leaves out is refused. An undo runs the inverse action, so name that too if undo should work:

```yaml
approvals:
  trust: marmot
  max_age: 15m
sources:
  github-acme:
    query: false
    origins: ["task_run:*"]
    actions:
      read_file: {}
      list_files: {}
      list_pull_requests: {}
      open_pull_request:
        approvals: 1
        params:
          repo: {values: [pipelines]}
          base: {values: [main]}
      comment_on_pull_request:
        approvals: 1
      close_pull_request:
        approvals: 0
      delete_comment:
        approvals: 0
```

This runner lets tasks read files and pull requests, open a pull request against `main` of `pipelines` and comment with 1 approval, and close what they opened or delete what they wrote with none.

## Triggers

The plugin has none. Webhooks would need an endpoint GitHub can reach, which a runner usually is not.
