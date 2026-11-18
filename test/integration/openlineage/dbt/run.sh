#!/usr/bin/env bash
# Runs the dbt shop project through dbt-ol so OpenLineage events go to $OPENLINEAGE_URL.
#
# Env:
#   OPENLINEAGE_URL        required, base URL (the client appends /api/v1/lineage)
#   OPENLINEAGE_API_KEY    optional, sent as a bearer token
#   OPENLINEAGE_NAMESPACE  optional, job namespace (default dbt-shop)
#   DBT_OL_FAIL=1          optional, also builds a broken model so a FAIL event is emitted
set -euo pipefail

cd "$(dirname "$0")"

: "${OPENLINEAGE_URL:?OPENLINEAGE_URL must be set}"
export OPENLINEAGE_URL
export OPENLINEAGE_NAMESPACE="${OPENLINEAGE_NAMESPACE:-dbt-shop}"
if [[ -n "${OPENLINEAGE_API_KEY:-}" ]]; then export OPENLINEAGE_API_KEY; fi
export DBT_PROFILES_DIR="$PWD"
export DBT_SEND_ANONYMOUS_USAGE_STATS=false
export DO_NOT_TRACK=1

if [[ ! -x .venv/bin/dbt-ol ]] || ! cmp -s requirements.txt .venv/requirements.txt; then
  rm -rf .venv
  uv venv --quiet --python 3.12 .venv
  uv pip install --quiet --python .venv/bin/python -r requirements.txt
  cp requirements.txt .venv/requirements.txt
fi
export PATH="$PWD/.venv/bin:$PATH"

# Start from an empty warehouse so every run emits the same events.
rm -rf warehouse target target-fail logs
mkdir -p warehouse

dbt-ol seed
dbt-ol run
dbt-ol test

if [[ "${DBT_OL_FAIL:-0}" == "1" ]]; then
  # Keep the manifest from the successful run; the failing invocation writes its own into a separate target dir.
  if dbt-ol run --vars '{include_broken: true}' --select broken_orders --target-path target-fail; then
    echo "expected broken_orders to fail" >&2
    exit 1
  fi
  echo "broken_orders failed as expected"
fi

echo "manifest: $PWD/target/manifest.json"
