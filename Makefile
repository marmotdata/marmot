.PHONY: swagger build run test clean dev release docker-build dev-deps generate generate-operator lint frontend-build actionlint frontend-lint frontend-typecheck fix api-client \
	sdk sdk-generate sdk-test sdk-build sdk-lint sdk-clean \
	sdk-py sdk-py-deps sdk-py-install sdk-py-generate sdk-py-lint sdk-py-test sdk-py-build sdk-py-clean

# Build variables
BINARY_NAME=marmot
GO_FILES=$(shell find . -name '*.go')
VERSION ?= $(shell git describe --tags --always --dirty --match=v* 2> /dev/null || echo "v0.0.0")
LDFLAGS_VERSION=-X "github.com/marmotdata/marmot/internal/cmd.Version=$(VERSION)"

swagger:
	swag init -d internal/api --generalInfo v1/server.go --parseDependency --output docs

build:
	go build -o bin/$(BINARY_NAME) cmd/main.go

dev: swagger build
	MARMOT_LOGGING_LEVEL=debug MARMOT_SERVER_ALLOW_UNENCRYPTED=true MARMOT_TELEMETRY_ENABLED=false ./bin/$(BINARY_NAME) run

frontend-build:
	cd web/marmot && pnpm install && node scripts/generate-icon-bundle.mjs && pnpm build
	mkdir -p internal/staticfiles/build
	cp -r web/marmot/build/* internal/staticfiles/build/

release: clean swagger frontend-build
	go build -tags=production -ldflags '$(LDFLAGS_VERSION)' -o bin/$(BINARY_NAME) cmd/main.go
	rm -rf internal/staticfiles/build

test:
	go test -v ./...

e2e-test: build test api-client
	cd test/e2e && go test -v -timeout 1h ./...

clean:
	rm -rf bin/ internal/static/build/
	go clean

generate:
	# Cleanup old docs before generating (top-level files only, preserves subdirectories)
	find web/docs/docs/Plugins -maxdepth 1 -type f ! -name "index.md" ! -name "_category_.json" -delete
	go generate ./...

CONTROLLER_GEN ?= $$(go env GOPATH)/bin/controller-gen

generate-operator:
	$(CONTROLLER_GEN) object paths=./internal/operator/api/...
	$(CONTROLLER_GEN) crd paths=./internal/operator/api/... output:crd:dir=charts/marmot/crds

lint: frontend-lint
	$$(go env GOPATH)/bin/golangci-lint run --config=./.github/.golangci.yaml ./... -v

frontend-lint:
	cd web/marmot && pnpm install && pnpm run lint

frontend-typecheck:
	cd web/marmot && pnpm install && pnpm run check

fix:
	cd web/marmot && pnpm run format

actionlint:
	actionlint

docker-build:
	docker build -t marmot -f deployments/docker/Dockerfile.backend .

dev-deps:
	curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/HEAD/install.sh | sh -s -- -b $$(go env GOPATH)/bin v2.9.0
	go install github.com/swaggo/swag/cmd/swag@latest
	go install github.com/rhysd/actionlint/cmd/actionlint@latest

api-client: swagger
	rm -rf client/client client/models
	swagger generate client -f docs/swagger.yaml -A marmot --target client
	cd client && go mod tidy

# =====================================================================
# SDK targets
#
# Per-language: sdk-<lang>-{deps,install,generate,lint,test,build,clean}
# Umbrellas:    sdk, sdk-{generate,lint,test,build,clean}
# CI uses the per-language targets; humans use the umbrellas.
# =====================================================================

# Umbrellas (extend with sdk-go-* and sdk-ts-* once those SDKs land).
sdk: sdk-py
sdk-generate: sdk-py-generate
sdk-lint: sdk-py-lint
sdk-test: sdk-py-test
sdk-build: sdk-py-build
sdk-clean: sdk-py-clean

# --- Python SDK ---
SDK_PY_DIR := sdk/python

# Install uv if missing. Idempotent; CI runners with uv preinstalled skip the curl.
sdk-py-deps:
	@command -v uv >/dev/null 2>&1 || (echo "Installing uv..." && curl -LsSf https://astral.sh/uv/install.sh | sh)

# Sync the SDK's own venv + dev deps. Run once before any other sdk-py target.
sdk-py-install: sdk-py-deps
	cd $(SDK_PY_DIR) && uv sync --all-extras

# Regenerate the typed client from docs/swagger.yaml. Depends on swagger so
# server-side changes flow through automatically. _gen/ is gitignored; CI must
# regenerate before lint/test/build.
sdk-py-generate: swagger sdk-py-install
	cd $(SDK_PY_DIR) && rm -rf src/marmot/_gen && \
		uv run openapi-python-client generate \
			--path ../../docs/swagger.yaml \
			--config codegen.yaml \
			--overwrite \
			--meta none \
			--output-path src/marmot/_gen

sdk-py-lint: sdk-py-install
	cd $(SDK_PY_DIR) && uv run ruff check . && uv run ruff format --check .

sdk-py-test: sdk-py-generate
	cd $(SDK_PY_DIR) && uv run pytest

sdk-py-build: sdk-py-generate
	cd $(SDK_PY_DIR) && uv build

sdk-py-clean:
	rm -rf $(SDK_PY_DIR)/src/marmot/_gen $(SDK_PY_DIR)/dist $(SDK_PY_DIR)/.venv

chart-test:
	docker run ${DOCKER_ARGS} --user root --entrypoint /bin/sh --rm -v $(CURDIR):/charts -w /charts helmunittest/helm-unittest:3.17.3-0.8.2 /charts/.github/test.sh

chart-lint:
	docker run ${DOCKER_ARGS} --env GIT_SAFE_DIR="true" --entrypoint /bin/sh --rm -v $(CURDIR):/charts -w /charts quay.io/helmpack/chart-testing:v3.13.0 /charts/.github/lint.sh
