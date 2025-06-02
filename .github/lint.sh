#!/bin/bash

CT_ARGS=""
GIT_SAFE_DIR="false"

if [ "$GIT_SAFE_DIR" != "true" ]; then
    git config --global --add safe.directory /charts
fi

ct lint --config=./.github/chart-testing.yaml
