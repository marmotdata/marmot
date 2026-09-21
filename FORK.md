# This fork

`dgu-development/marmot` is a public MIT fork of [marmotdata/marmot](https://github.com/marmotdata/marmot). It exists so reusable kernel work (registry, native Discover, scoped auth) can land without putting corporate YAML or Telefónica overlay in this tree.

The consuming distribution pins a **SHA**, never a floating tag. Corporate profiles and locales live there, not here.

## Branches

```
marmotdata/marmot          main     ← upstream
dgu-development/marmot     main     ← fast-forward only copy of upstream
dgu-development/marmot     dgu      ← product line (PRs land here)
                           feature/*  ← from dgu (fork-only) or from main (upstream-candidate)
```

| Branch | Role |
| --- | --- |
| `main` | Mirror of `marmotdata/marmot` `main`. No unique commits. Not the product. |
| `dgu` | Long-lived product line. Default branch for issues, PR templates, and CODEOWNERS. Consuming distributions pin a SHA of this branch. |
| `feature/*` | Reviewable work. Fork-only: branch from `dgu`, PR to `dgu`. Upstream-candidate: branch from `main`, PR to marmotdata **and** land the same patch on `dgu`. |

Do not use GitHub **Sync fork**. That updates the default branch; after `dgu` is default, it would merge upstream into the product line. Refresh `main` with `.github/workflows/sync-upstream-main.yml` (`workflow_dispatch` or the weekly schedule) or:

```bash
git fetch upstream
git push origin upstream/main:main   # must be a fast-forward
```

Integrate upstream into the product line with a PR **`main` → `dgu`**. Resolve conflicts there. Do not rebase published `dgu` history once a SHA is pinned in a distribution.

## What belongs where

| Here (`dgu`) | Not here |
| --- | --- |
| Generic Go/API/Discover, goose migrations, tests | `metamodel/dgu-core.yaml`, ES/EN catalogues |
| Fork GitHub forms, CODEOWNERS, this file | Overlay, `/dgu/*` modules, `kernel.lock.json` |
| PRs labelled `fork-only` or `upstream-candidate` | Secrets, tenant data |

Upstream-candidate: branch from `main` (or `upstream/main`). Open a PR to **marmotdata/marmot** and land the **same** patch on `dgu` (cherry-pick or a second PR). Do not rewrite the change twice. After upstream merges, fast-forward `origin/main`, then open PR **`main` → `dgu`** so the product line also picks up the rest of Marmot.

## Labels

- `fork-only` — will not be proposed upstream
- `upstream-candidate` — stacked on `main` for marmotdata

## Pin

Distributions record `sourceReviewed` as the shipped commit of **this** fork (on `dgu`) and keep the upstream base SHA separately. An empty `patches` array means nothing extra is shipped yet; it does not forbid work on `dgu`.
