.PHONY: swagger build run test clean dev release docker-build dev-deps generate-operator lint server-lint frontend-build actionlint frontend-lint frontend-typecheck fix \
	sdk sdk-generate sdk-test sdk-build sdk-lint sdk-clean \
	sdk-go sdk-go-generate sdk-go-lint sdk-go-test sdk-go-build sdk-go-clean \
	sdk-py sdk-py-deps sdk-py-install sdk-py-generate sdk-py-lint sdk-py-test sdk-py-build sdk-py-clean \
	sdk-ts sdk-ts-deps sdk-ts-install sdk-ts-generate sdk-ts-lint sdk-ts-test sdk-ts-build sdk-ts-clean

# Build variables
BINARY_NAME=marmot
GO_FILES=$(shell find . -name '*.go')
VERSION ?= $(shell git describe --tags --always --dirty --match=v* 2> /dev/null || echo "v0.0.0")
LDFLAGS_VERSION=-X "github.com/marmotdata/marmot/internal/cmd.Version=$(VERSION)"

SWAG_VERSION := v1.16.6
GO_SWAGGER_VERSION := v0.36.5
SWAG := $(shell go env GOPATH)/bin/swag

$(SWAG):
	go install github.com/swaggo/swag/cmd/swag@$(SWAG_VERSION)

GO_SWAGGER := $(shell go env GOPATH)/bin/swagger

$(GO_SWAGGER):
	go install github.com/go-swagger/go-swagger/cmd/swagger@$(GO_SWAGGER_VERSION)

swagger: $(SWAG)
	$(SWAG) init -d internal/api --generalInfo v1/server.go --parseDependency --output docs
	rm -f $(SDK_OPENAPI3)

build:
	go build -ldflags '-s -w' -o bin/$(BINARY_NAME) cmd/main.go

dev: swagger
	go build -ldflags '-s -w' -tags=swagger -o bin/$(BINARY_NAME) cmd/main.go
	MARMOT_LOGGING_LEVEL=debug MARMOT_SERVER_ALLOW_UNENCRYPTED=true MARMOT_TELEMETRY_ENABLED=false ./bin/$(BINARY_NAME) run

frontend-build:
	cd web/marmot && pnpm install && node scripts/generate-icon-bundle.mjs && pnpm build
	rm -rf internal/staticfiles/build
	mkdir -p internal/staticfiles/build
	cp -r web/marmot/build/* internal/staticfiles/build/

release: clean swagger frontend-build
	go build -tags=production -ldflags '-s -w $(LDFLAGS_VERSION)' -o bin/$(BINARY_NAME) cmd/main.go
	rm -rf internal/staticfiles/build

test:
	go test -v ./...

e2e-test: build test sdk-go-generate
	cd test/e2e && go test -v -timeout 1h ./...

clean:
	rm -rf bin/ internal/static/build/
	go clean

CONTROLLER_GEN ?= $$(go env GOPATH)/bin/controller-gen

generate-operator:
	$(CONTROLLER_GEN) object paths=./internal/operator/api/...
	$(CONTROLLER_GEN) crd paths=./internal/operator/api/... output:crd:dir=charts/marmot/crds

lint: frontend-lint sdk-go-lint sdk-py-lint server-lint

server-lint:
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
	go install github.com/swaggo/swag/cmd/swag@$(SWAG_VERSION)
	go install github.com/go-swagger/go-swagger/cmd/swagger@$(GO_SWAGGER_VERSION)
	go install github.com/rhysd/actionlint/cmd/actionlint@latest

sdk: sdk-generate
sdk-generate: sdk-go-generate sdk-py-generate sdk-ts-generate
sdk-lint: sdk-go-lint sdk-py-lint sdk-ts-lint
sdk-test: sdk-go-test sdk-py-test sdk-ts-test
sdk-build: sdk-go-build sdk-py-build sdk-ts-build
sdk-clean: sdk-go-clean sdk-py-clean sdk-ts-clean

SDK_GO_DIR := sdk/go
SDK_PY_DIR := sdk/python
SDK_TS_DIR := sdk/ts
SDK_OPENAPI3 := docs/.openapi3.yaml

sdk-go-generate: swagger $(GO_SWAGGER)
	rm -rf $(SDK_GO_DIR)/internal/gen
	mkdir -p $(SDK_GO_DIR)/internal/gen
	$(GO_SWAGGER) generate client -f docs/swagger.yaml -A marmot --target $(SDK_GO_DIR)/internal/gen
	cd $(SDK_GO_DIR) && go mod tidy

sdk-go-lint:
	cd $(SDK_GO_DIR) && $$(go env GOPATH)/bin/golangci-lint run --config=$(CURDIR)/.github/.golangci.yaml ./... -v

sdk-go-test:
	cd $(SDK_GO_DIR) && go test ./...

sdk-go-build:
	cd $(SDK_GO_DIR) && go build ./...

sdk-go-clean:
	rm -rf $(SDK_GO_DIR)/internal/gen

$(SDK_OPENAPI3): docs/swagger.yaml
	npx --yes swagger2openapi@7 docs/swagger.yaml --outfile $(SDK_OPENAPI3) --yaml

sdk-py-deps:
	@command -v uv >/dev/null 2>&1 || (echo "Installing uv..." && curl -LsSf https://astral.sh/uv/install.sh | sh)

sdk-py-install: sdk-py-deps
	cd $(SDK_PY_DIR) && uv sync --all-extras

sdk-py-generate: swagger sdk-py-install $(SDK_OPENAPI3)
# when bumping openapi-generator-cli, check for any changes in dependencies:
# generate to tmp path with `generateSourceCodeOnly=true` and cross-reference marmots/generated pyproject.toml
# when adding an API version, add namespacing
	rm -rf sdk/python/src/marmot/generated && \
	docker run --rm -v "$${PWD}:/local" openapitools/openapi-generator-cli:v7.24.0 generate \
		-i /local/$(SDK_OPENAPI3) \
		-g python \
		-o /local/_tmp \
		--global-property apiDocs=false,modelDocs=false \
		--http-user-agent marmot-sdk-py \
		--library httpx \
		--additional-properties=lazyImports=true,generateSourceCodeOnly=true,supportHttpxSync=true,packageName=marmot.generated
	mv _tmp/marmot/generated sdk/python/src/marmot/ && rm -rf _tmp/
	make sdk-py-format

sdk-py-format: sdk-py-install
	cd $(SDK_PY_DIR) && uv run ruff check . --fix --unsafe-fixes
	cd $(SDK_PY_DIR) && uv run ruff format

sdk-py-lint: sdk-py-install
	cd $(SDK_PY_DIR) && uv run ruff check .
	cd $(SDK_PY_DIR) && uv run ruff format --check .
	cd $(SDK_PY_DIR) && uv run mypy src/marmot
	cd $(SDK_PY_DIR) && uv run mypy examples
	cd $(SDK_PY_DIR) && uv run pip-audit --skip-editable

sdk-py-test: sdk-py-install
	cd $(SDK_PY_DIR) && uv run pytest

sdk-py-build:
	cd $(SDK_PY_DIR) && uv build

sdk-py-clean:
	rm -rf $(SDK_PY_DIR)/dist

sdk-ts-deps:
	@command -v pnpm >/dev/null 2>&1 || (echo "pnpm not installed; install via 'npm i -g pnpm' or corepack" && exit 1)

sdk-ts-install: sdk-ts-deps
	cd $(SDK_TS_DIR) && pnpm install --frozen-lockfile=false

sdk-ts-generate: swagger sdk-ts-install $(SDK_OPENAPI3)
	cd $(SDK_TS_DIR) && rm -rf src/_gen && mkdir -p src/_gen && \
		pnpm exec openapi-typescript ../../$(SDK_OPENAPI3) -o src/_gen/schema.ts && \
		node scripts/generate-models.mjs

sdk-ts-lint: sdk-ts-install
	cd $(SDK_TS_DIR) && pnpm run lint

sdk-ts-test: sdk-ts-generate
	cd $(SDK_TS_DIR) && pnpm run test

sdk-ts-build: sdk-ts-generate
	cd $(SDK_TS_DIR) && pnpm run build

sdk-ts-clean:
	rm -rf $(SDK_TS_DIR)/src/_gen $(SDK_TS_DIR)/dist $(SDK_TS_DIR)/node_modules

chart-test:
	docker run ${DOCKER_ARGS} --user root --entrypoint /bin/sh --rm -v $(CURDIR):/charts -w /charts helmunittest/helm-unittest:3.17.3-0.8.2 /charts/.github/test.sh

chart-lint:
	docker run ${DOCKER_ARGS} --env GIT_SAFE_DIR="true" --entrypoint /bin/sh --rm -v $(CURDIR):/charts -w /charts quay.io/helmpack/chart-testing:v3.13.0 /charts/.github/lint.sh
