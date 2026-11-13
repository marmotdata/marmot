#!/usr/bin/env bash
# Tags every plugin that changed since its last release, and pushes the tags.
# The next version comes from the commits since that tag: any "feat" commit
# touching the plugin is a minor bump, anything else a patch. Prints the plan
# by default; pass --push to tag and push. Tags go one at a time, because
# GitHub starts no workflow for a push of more than three tags.
#
#   scripts/release-plugins.sh            # show what would be released
#   scripts/release-plugins.sh --push     # tag and push
#   scripts/release-plugins.sh --push kafka mysql   # only these plugins
set -euo pipefail

cd "$(git rev-parse --show-toplevel)"

push=false
if [[ "${1:-}" == "--push" ]]; then
  push=true
  shift
fi
only=("$@")

if $push; then
  [[ "$(git rev-parse --abbrev-ref HEAD)" == "main" ]] || { echo "Switch to main first." >&2; exit 1; }
  git fetch -q origin main --tags
  [[ "$(git rev-parse HEAD)" == "$(git rev-parse origin/main)" ]] || { echo "main is not up to date with origin/main." >&2; exit 1; }
fi

planned=()
for dir in plugins/*/; do
  name=$(basename "$dir")
  if ((${#only[@]})) && [[ ! " ${only[*]} " == *" ${name} "* ]]; then
    continue
  fi
  last=$(git tag -l "plugins/${name}/v*" --sort=-v:refname | head -n 1)
  if [[ -z "$last" ]]; then
    next="v0.1.0"
    reason="first release"
  else
    commits=$(git log --format=%s "${last}..HEAD" -- "$dir")
    [[ -z "$commits" ]] && continue
    IFS=. read -r major minor patch <<<"${last##*/v}"
    if grep -qE '^feat(\(|:|!)' <<<"$commits"; then
      next="v${major}.$((minor + 1)).0"
      reason="new features"
    else
      next="v${major}.${minor}.$((patch + 1))"
      reason="fixes and dependencies"
    fi
  fi
  printf '%-14s %-8s -> %-8s %s\n' "$name" "${last##*/}" "$next" "$reason"
  planned+=("plugins/${name}/${next}")
done

if ((${#planned[@]} == 0)); then
  echo "Nothing to release."
  exit 0
fi

if ! $push; then
  echo
  echo "${#planned[@]} plugins. Run again with --push to release them."
  exit 0
fi

for tag in "${planned[@]}"; do
  git tag "$tag"
  git push -q origin "$tag"
  echo "pushed $tag"
  sleep 2
done
echo "Done. Each tag runs the Release Plugin workflow on the Actions tab."
