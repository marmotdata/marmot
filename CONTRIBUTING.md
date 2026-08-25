# Contributing to Marmot

Thanks for your interest in contributing to Marmot! We welcome contributions of all kinds - from bug fixes and documentation improvements to new features and plugins.

## Getting Started

Before contributing, please:

1. **Set up your development environment** - Follow the [local development guide](https://marmotdata.io/docs/Develop/local-development) to get Marmot running locally
2. **Read the documentation** - Familiarise yourself with how Marmot works at [marmotdata.io/docs](https://marmotdata.io/docs)
3. **Check existing issues** - Browse [open issues](https://github.com/marmotdata/marmot/issues) to see what needs work

## Ways to Contribute

### Reporting Bugs

Found a bug? Help us fix it by:

1. Checking if the issue already exists in [GitHub Issues](https://github.com/marmotdata/marmot/issues)
2. If not, create a new issue with:
   - A clear, descriptive title
   - Steps to reproduce the bug
   - Expected vs actual behaviour
   - Your environment
   - Screenshots if applicable

### Suggesting Features

Have an idea for a new feature?

1. Check existing [feature requests](https://github.com/marmotdata/marmot/issues?q=is%3Aissue+label%3A%22kind%2Ffeature%22)
2. Open a new issue with:
   - A clear description of the feature
   - The problem it solves

### Contributing Code

#### Before You Start

- Check if there's an existing issue for what you want to work on
- If not, create one to discuss your approach
- Wait for feedback before investing significant time
- Fork the repository and set up your [development environment](https://marmotdata.io/docs/Develop/local-development)

### Building Plugins

Want to add support for a new data source? Check out the [plugin development guide](https://marmotdata.io/docs/Develop/creating-plugins) to learn how to build custom plugins.

### Improving Documentation

Documentation improvements are always welcome! You can:

- Fix typos or unclear explanations
- Add examples and use cases
- Improve API documentation
- Create tutorials or guides

Documentation lives in the `web/docs` directory.

## Code Review

All submissions require review. We aim to:

- Respond to pull requests within 48 hours
- Provide constructive, helpful feedback
- Work with you to get your contribution merged

### Bot commands

We use [Prow](https://docs.prow.k8s.io/)-style slash commands, via
[`cncf/prow-github-actions`](https://github.com/cncf/prow-github-actions). Put a command on its
own line in a comment.

Anyone can use these on their own pull request:

| Command | Effect |
| --- | --- |
| `/assign` / `/unassign [@user]` | Assign yourself, or someone who is already involved |
| `/cc` / `/uncc [@user]` | Request or withdraw a review request |

Maintainers additionally have `/lgtm`, `/hold`, `/area`, `/kind`, `/priority`, `/remove`,
`/retitle`, `/milestone`, `/close` and `/reopen`.

There is deliberately no `/approve`. Approving a pull request is a UI action, so that the
approval is attributed to the maintainer who actually gave it rather than to a bot.

How a PR merges. Two things have to be true: a maintainer has commented `/lgtm`, which adds the
`lgtm` label, and the PR satisfies branch protection, meaning required checks green and an
approving review from someone other than the author. `marmot-ci-robot` then squash-merges it on
the next hourly pass. `/hold` blocks the merge until someone comments `/hold cancel`.

Pushing a new commit removes the `lgtm` label and dismisses the approval, so an updated PR needs
a fresh review. On pull requests from forks the label sometimes survives the push, because the
bot cannot write to fork PRs. The dismissed approval still holds the merge, so this is cosmetic.

One sharp edge worth knowing: the bot matches commands anywhere in a comment, not just at the
start of a line. Quoting someone else's `/lgtm` in a reply will re-apply it, and writing prose
like "what /kind of error" will apply a label. If you need to mention a command without running
it, break it up (`/ lgtm`).

## Questions?

- Ask in [GitHub Discussions](https://github.com/marmotdata/marmot/discussions)
- Check the [documentation](https://marmotdata.io/docs)
- Open an issue if you're stuck

## License

By contributing to Marmot, you agree that your contributions will be licensed under the [MIT License](LICENSE).
