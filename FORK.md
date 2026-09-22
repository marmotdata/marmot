# This fork

`dgu-development/marmot` is a public MIT fork of [marmotdata/marmot](https://github.com/marmotdata/marmot). It exists so reusable kernel work (registry, native Discover, scoped auth) can land without putting corporate YAML or Telefónica overlay in this tree.

The consuming distribution pins a **SHA**, never a floating tag. Corporate profiles and locales live there, not here.

## Branches

```
marmotdata/marmot          main     ← upstream
dgu-development/marmot     main     ← fast-forward only mirror of upstream (reference only, never a branch point)
dgu-development/marmot     dgu      ← product line (PRs land here)
                           upstream/*  ← upstream-candidate, branched from a freshly fetched upstream/main
                           feature/*   ← fork-only, branched from dgu
```

| Branch | Role |
| --- | --- |
| `main` | Read-only mirror of `marmotdata/marmot` `main`. No unique commits. **Never branch from it** — it can be stale between syncs; kept only for browsing/diffing. |
| `dgu` | Long-lived product line. Default branch for issues, PR templates, and CODEOWNERS. Consuming distributions pin a SHA of this branch. |
| `upstream/*` | Upstream-candidate work. Branch from a freshly fetched `upstream/main`, PR to marmotdata/marmot. Rebase only — never merge `dgu`, `main`, or a fork branch into it. |
| `feature/*` | Fork-only work. Branch from `dgu`, PR to `dgu`. |

Do not use GitHub **Sync fork**, and do not open or merge PRs against `main` — it must stay bit-for-bit identical to `upstream/main`, not just fast-forward-able from it. Two things enforce that:

- [`.github/workflows/sync-upstream-main.yml`](.github/workflows/sync-upstream-main.yml) refreshes it (`workflow_dispatch` or the weekly schedule) by resetting to `upstream/main` and force-pushing, so it self-heals even if something landed on `main` by other means — it does not assume `main` was only ever fast-forwarded.
- [`.github/workflows/reject-prs-to-main.yml`](.github/workflows/reject-prs-to-main.yml) auto-closes any PR opened against `main`, whatever produced it.

Branch protection on `main` should have **no** required-PR rule and a push restriction allowing only the identity the sync workflow runs as (its `GITHUB_TOKEN`/App, or a bot PAT if the org disallows Actions in push-restriction allow-lists) — that's what actually stops a human's **Sync fork** click or a manual `git push origin main` from sneaking a merge in, rather than just asking people not to.

If `main` has already drifted (a Sync fork merge, a leftover PR, anything not from upstream), restore it by hand — a fast-forward push will refuse a diverged history, so this resets and force-pushes instead:

```bash
git fetch upstream
git checkout main
git reset --hard upstream/main
git push origin main --force-with-lease   # branch protection may need a one-off unlock for this push
```

The mirror is a convenience, **not** where `upstream/*` branches come from. Always branch from a fetch you just did:

```bash
git fetch upstream
git switch -c upstream/<slug> upstream/main
```

This avoids two failure modes seen before: branching from a stale local `main`, and branching from `dgu` by accident because it's this repo's default branch. Never run `git merge dgu` or `git merge main` inside an `upstream/*` branch — that drags fork-only commits (CODEOWNERS, issue templates, `FORK.md`, `.vscode/`) into a PR bound for marmotdata. Need the latest upstream mid-branch? `git rebase upstream/main` instead. `.github/workflows/guard-upstream-candidate.yml` enforces both rules — no merge commits, no fork-only paths — on every push to `upstream/**`, and blocks a PR that targets `dgu` straight from an `upstream/*` branch.

Backport the same patch to `dgu` by cherry-pick, not by opening the `upstream/*` branch itself as a second PR:

```bash
git switch -c backport/<slug> origin/dgu
git cherry-pick -x upstream/main..upstream/<slug>
```

## What belongs where

| Here (`dgu`) | Not here |
| --- | --- |
| Generic Go/API/Discover, goose migrations, tests | `metamodel/dgu-core.yaml`, ES/EN catalogues |
| Fork GitHub forms, CODEOWNERS, this file | Overlay, `/dgu/*` modules, `kernel.lock.json` |
| PRs labelled `fork-only` or `upstream-candidate` | Secrets, tenant data |

Anything in that "Fork GitHub forms, CODEOWNERS, this file" cell must never reach an `upstream/*` branch or a marmotdata PR — that's exactly what the guard workflow checks for.

Upstream-candidate: branch from a freshly fetched `upstream/main`. Open a PR to **marmotdata/marmot** and land the **same** patch on `dgu` by cherry-pick (see above). Do not rewrite the change twice. After upstream merges, fast-forward `origin/main`, then open PR **`main` → `dgu`** so the product line also picks up the rest of Marmot — that PR carries the rest of upstream, not the patch you already backported.

## Labels

- `fork-only` — will not be proposed upstream
- `upstream-candidate` — branched from a freshly fetched `upstream/main`, proposed to marmotdata/marmot

## Pin

Distributions record `sourceReviewed` as the shipped commit of **this** fork (on `dgu`) and keep the upstream base SHA separately. An empty `patches` array means nothing extra is shipped yet; it does not forbid work on `dgu`.
